/*
Copyright 2014 Rohith Jayawardene All rights reserved.

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

const (
	DOCUMENT_IPADDRESS = "ipaddress"
	DOCUMENT_PORT      = "port"
	DOCUMENT_TAGS      = "tags"
)

type EtcdDiscoveryService struct {
	client    *etcd.Client
	waitIndex uint64
}

func NewEtcdStore(config *config.Configuration) (DiscoveryStoreProvider, error) {
	glog.V(3).Infof("Creating a new Etcd client")
	uri := config.GetDiscoveryURI()
	urls := make([]string, 0)
	if uri.Host != "" {
		urls = append(urls, "http://"+uri.Host)
	}
	return &EtcdDiscoveryService{client: etcd.NewClient(urls)}, nil
}

func (e *EtcdDiscoveryService) List(si *services.Service) ([]services.Endpoint, error) {
	list := make([]services.Endpoint, 0)
	glog.V(5).Infof("Listing the nodes for service: %s, path: %s", si, si.Name)
	paths := make([]string, 0)
	/* step: we get a listing of all the nodes under or branch */
	paths, err := EtcdPathWalker(e.client, string(si.Name), &paths)
	if err != nil {
		glog.Errorf("Failed to generate a list of the tree nodes, error: %s", err)
		return nil, err
	}
	/* step: iterate the nodes and generate the services documents */
	for _, path := range paths {
		glog.V(5).Infof("Retrieving the service document from path: %s", path)
		response, err := e.client.Get(path, false, false)
		if err != nil {
			glog.Errorf("Failed to retrieve the service document, path: %s, error: %s", path, err)
			continue
		}
		/* step: convert the document into a record */
		if endpoint, err := e.GetServiceDocument([]byte(response.Node.Value)); err == nil {
			glog.V(5).Infof("Found endpoint, service: %s, endpoint: %s", si, endpoint)
			list = append(list, endpoint)
		}
	}
	return list, nil
}

/*
	Take the node value which should have json data, convert to a service document and attempt to extract the
	required fields
*/
func (e *EtcdDiscoveryService) GetServiceDocument(document []byte) (services.Endpoint, error) {
	glog.V(5).Infof("Attempting to decode the service document: %s", document)
	service := &EtcdServiceDocument{}
	if err := json.Unmarshal(document, &service); err != nil {
		glog.Errorf("Unable to decode the service document: %s", document)
		return "", err
	}
	/* step: validate the service document and format it */
	endpoint, err := IsValidServiceDocument(service)
	if err != nil {
		glog.Errorf("Invalid service document, error: %s", err)
		return "", err
	}
	glog.V(5).Infof("Service document decoded into endpoint: %s", endpoint)
	return endpoint, nil
}

func (e *EtcdDiscoveryService) Watch(si *services.Service) {
	if resp, err := e.client.Watch(si.Name, e.waitIndex, true, nil, nil); err != nil {
		glog.Error("etcd:", err)
	} else {
		e.waitIndex = resp.EtcdIndex + 1
	}
}

func EtcdPathWalker(client *etcd.Client, path string, paths *[]string) ([]string, error) {
	response, err := client.Get(path, false, true)
	if err != nil {
		return nil, errors.New("Unable to complete walking the tree" + err.Error())
	}
	for _, node := range response.Node.Nodes {
		if node.Dir {
			EtcdPathWalker(client, node.Key, paths)
		} else {
			glog.Infof("Found node: %s appeding now", node.Key)
			*paths = append(*paths, node.Key)
		}
	}
	return *paths, nil
}

func IsValidServiceDocument(document *EtcdServiceDocument) (endpoint services.Endpoint, err error) {
	if document.IPaddress == "" {
		return endpoint, errors.New("Invalid service registration, document does not contain a ipaddress")
	}
	if document.Port == "" {
		return endpoint, errors.New("Invalid service registration, document does not contain a port")
	}
	return services.Endpoint(fmt.Sprintf("%s:%s", document.IPaddress, document.Port)), nil
}
