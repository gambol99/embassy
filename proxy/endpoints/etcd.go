/*
Copyright 2014 Rohith All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package endpoints

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"strings"
	"time"

	"github.com/gambol99/embassy/proxy/services"
	"github.com/gambol99/embassy/utils"

	"github.com/coreos/go-etcd/etcd"
	"github.com/golang/glog"
)

type EtcdClient struct {
	// the etcd http client
	client *etcd.Client
	// the shutdown channel for the service
	shutdown_channel utils.ShutdownSignalChannel
	// a kill switch
	kill_off bool
}

var EtcdOptions struct {
	cert_file, key_file, cacert_file string
}

func init() {
	flag.StringVar(&EtcdOptions.cert_file, "etcd-cert", "", "the etcd certificate file (optional)")
	flag.StringVar(&EtcdOptions.key_file, "etcd-keycert", "", "the etcd key certificate file (optional)")
	flag.StringVar(&EtcdOptions.cacert_file, "etcd-cacert", "", "the etcd ca certificate file (optional)")
}

const (
	ETCD_PREFIX = "etcd://"
)

func NewEtcdStore(uri string) (EndpointsProvider, error) {
	glog.V(3).Infof("Creating a Etcd client, hosts: %s", uri)
	/* step: get the etcd nodes from the discovery uri */
	service := new(EtcdClient)
	if EtcdOptions.cert_file != "" {
		/* step: create a tls connection */
		client, err := etcd.NewTLSClient(GetEtcdHosts(uri), EtcdOptions.cert_file,
			EtcdOptions.key_file, EtcdOptions.cacert_file)
		if err != nil {
			glog.Errorf("Failed to create a TLS connection to etcd: %s, error: %s", uri, err)
			return nil, err
		}
		service.client = client
	} else {
		service.client = etcd.NewClient(GetEtcdHosts(uri))
	}
	service.shutdown_channel = make(utils.ShutdownSignalChannel)
	service.kill_off = false
	return service, nil
}

func (r *EtcdClient) Close() {
	glog.Infof("Request to shutdown the endpoints agent")
	r.kill_off = true
	r.shutdown_channel <- true
}

func (e *EtcdClient) Watch(si *services.Service) (updates EndpointEventChannel, err error) {
	// channel to send back events to the endpoints store
	endpointUpdateChannel := make(EndpointEventChannel, 5)
	// channel to receive events from the watcher
	endpointWatchChannel := make(chan *etcd.Response)
	// channel to close the watcher
	stopChannel := make(chan bool)

	go func() {
		// step: start watching the endpoints
		go e.WaitForChanges(si.Name, endpointWatchChannel, stopChannel)
		// step: we wait for events from the above
		for {
			select {
			case update := <-endpointWatchChannel:
				var event EndpointEvent
				event.ID = update.Node.Key
				switch update.Action {
				case "set":
					event.Action = ENDPOINT_CHANGED
				case "delete":
					event.Action = ENDPOINT_REMOVED
				default:
					glog.Errorf("Unknown action recieved from etcd: %V", update)
					continue
				}
				// send the event upstream to endpoints store
				endpointUpdateChannel <- event

			case <-e.shutdown_channel:
				glog.Infof("Shutting down the watcher on service: %s", si)
				stopChannel <- true
				return
			}
		}
	}()
	return endpointUpdateChannel, nil
}

func (r *EtcdClient) WaitForChanges(path string, update_channel chan *etcd.Response, stop_channel chan bool) {
	for {
		glog.V(5).Infof("Waiting on endpoints for service path: %s to change", path)
		response, err := r.client.Watch(path, uint64(0), true, nil, stop_channel)
		if err != nil {
			if r.kill_off {
				glog.Infof("Quitting the watcher on service path: %s", path)
				return
			} else {
				glog.Errorf("Etcd client for service path: %s recieved an error: %s", path, err)
				time.Sleep(3 * time.Second)
				continue
			}
		}
		/* else we have a good response - lets check if it's a directory change */
		if response.Node.Dir == false {
			glog.V(7).Infof("Changed occured on path: %s", path)
			update_channel <- response
		}
	}
}

func (e *EtcdClient) List(si *services.Service) ([]Endpoint, error) {
	list := make([]Endpoint, 0)
	glog.V(5).Infof("Listing the container nodes for service: %s, path: %s", si, si.Name)

	/* step: we get a listing of all the nodes under or branch */
	paths := make([]string, 0)
	paths, err := e.Paths(string(si.Name), &paths)
	if err != nil {
		glog.Errorf("Failed to walk the paths for service: %s, error: %s", si, err)
		return nil, err
	}
	/* step: iterate the nodes and generate the services documents */
	for _, service_path := range paths {
		glog.V(5).Infof("Retrieving service document on path: %s", service_path)
		response, err := e.client.Get(service_path, false, false)
		if err != nil {
			glog.Errorf("Failed to get service document at path: %s, error: %s", service_path, err)
			continue
		}
		/* step: convert the document into a record */
		document, err := NewEtcdDocument([]byte(response.Node.Value))
		if err != nil {
			glog.Errorf("Unable to convert the response to service document, error: %s", err)
			continue
		}
		list = append(list, document.ToEndpoint())
	}
	return list, nil
}

func (e *EtcdClient) Paths(path string, paths *[]string) ([]string, error) {
	response, err := e.client.Get(path, false, true)
	if err != nil {
		return nil, errors.New("Unable to complete walking the tree" + err.Error())
	}
	for _, node := range response.Node.Nodes {
		if node.Dir {
			e.Paths(node.Key, paths)
		} else {
			glog.Infof("Found service container: %s appending now", node.Key)
			*paths = append(*paths, node.Key)
		}
	}
	return *paths, nil
}

func GetEtcdHosts(uri string) []string {
	hosts := make([]string, 0)
	protocol := "http"
	if EtcdOptions.cacert_file != "" {
		protocol = "https"
	}
	for _, etcd_host := range strings.Split(uri, ",") {
		if strings.HasPrefix(etcd_host, ETCD_PREFIX) {
			etcd_host = strings.TrimPrefix(etcd_host, ETCD_PREFIX)
		}
		hosts = append(hosts, fmt.Sprintf("%s://%s", protocol, etcd_host))
	}
	return hosts
}

/* --------- Service Document Decoding ------------ */

type EtcdServiceDocument struct {
	IPaddress string   `json:"ipaddress"`
	HostPort  string   `json:"host_port"`
	Port      string   `json:"port"`
	Tags      []string `json:"tags"`
}

func NewEtcdDocument(data []byte) (*EtcdServiceDocument, error) {
	document := &EtcdServiceDocument{}
	if err := json.Unmarshal(data, &document); err != nil {
		glog.Errorf("Unable to decode the service document: %s, error: %s", document, err)
		return nil, err
	}
	if err := document.IsValid(); err != nil {
		return nil, err
	}
	return document, nil
}

func (e EtcdServiceDocument) ToEndpoint() Endpoint {
	/* check: since most registration / discovery uses port rather than host_port */
	port := ""
	if e.HostPort != "" {
		port = e.HostPort
	} else {
		port = e.Port
	}
	return Endpoint(fmt.Sprintf("%s:%s", e.IPaddress, port))
}

func (e EtcdServiceDocument) IsValid() error {
	if e.IPaddress == "" || e.Port == "" {
		return errors.New("Invalid service document, does not contain a ipaddress and port")
	}
	return nil
}
