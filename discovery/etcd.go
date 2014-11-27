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

package discovery

/*
	The node value in etcd must be a json string which contains at the very least, 'ipaddress','port'
*/

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/coreos/go-etcd/etcd"
	"github.com/gambol99/embassy/config"
	"github.com/gambol99/embassy/services"
	"github.com/golang/glog"
)

type EtcdServiceDocument struct {
	IPaddress string   `json:"ipaddress"`
	Port      string   `json:"host_port"`
	Tags      []string `json:"tags"`
}

func NewEtcdDocument(data []byte) (*EtcdServiceDocument, error) {
	document := &EtcdServiceDocument{}
	if err := json.Unmarshal(data, &document); err != nil {
		glog.Errorf("Unable to decode the service document: %s", document)
		return nil, err
	}
	/* step: check if the document is valid */
	if err := document.IsValid(); err != nil {
		return nil, err
	}
	return document, nil
}

func (e EtcdServiceDocument) ToEndpoint() services.Endpoint {
	return services.Endpoint(fmt.Sprintf("%s:%s", e.IPaddress, e.Port))
}

func (e EtcdServiceDocument) IsValid() error {
	if e.IPaddress == "" || e.Port == "" {
		return errors.New("Invalid service document, does not contain a ipaddress and port")
	}
	return nil
}

type EtcdDiscoveryService struct {
	client    *etcd.Client
	waitIndex uint64
}

const ETCD_PREFIX = "etcd://"

func NewEtcdStore(cfg *config.Configuration) (DiscoveryStoreProvider, error) {
	glog.V(3).Infof("Creating a Etcd client, hosts: %s", cfg.DiscoveryURI)
	/* step: get the etcd nodes from the dicovery uri */
	return &EtcdDiscoveryService{etcd.NewClient(GetEtcdHosts(cfg.DiscoveryURI)), 0}, nil
}

func (e *EtcdDiscoveryService) List(si *services.Service) ([]services.Endpoint, error) {
	list := make([]services.Endpoint, 0)
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
			glog.Errorf("Failed to retrieve service document at path: %s, error: %s", service_path, err)
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

func (e *EtcdDiscoveryService) Watch(si *services.Service) error {
	if resp, err := e.client.Watch(si.Name, e.waitIndex, true, nil, nil); err != nil {
		glog.Error("etcd:", err)
		return err
	} else {
		e.waitIndex = resp.EtcdIndex + 1
	}
	return nil
}

func (e *EtcdDiscoveryService) Paths(path string, paths *[]string) ([]string, error) {
	response, err := e.client.Get(path, false, true)
	if err != nil {
		return nil, errors.New("Unable to complete walking the tree" + err.Error())
	}
	for _, node := range response.Node.Nodes {
		if node.Dir {
			e.Paths(node.Key, paths)
		} else {
			glog.Infof("Found service container: %s appeding now", node.Key)
			*paths = append(*paths, node.Key)
		}
	}
	return *paths, nil
}

func GetEtcdHosts(uri string) []string {
	hosts := make([]string, 0)
	for _, etcd_host := range strings.Split(uri, ",") {
		if strings.HasPrefix(etcd_host, ETCD_PREFIX) {
			etcd_host = strings.TrimPrefix(etcd_host, ETCD_PREFIX)
		}
		hosts = append(hosts, "http://"+etcd_host)
	}
	return hosts
}
