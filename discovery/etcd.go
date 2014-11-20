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
	IPaddress string   `json:"host"`
	Port      int      `json:"port"`
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

func (etcd *EtcdDiscoveryService) List(si *services.Service) ([]services.Endpoint, error) {
	list := make([]services.Endpoint, 0)
	if resp, err := etcd.client.Get(string(si.Name), false, true); err != nil {
		glog.Error("etcd:", err)
		return nil, errors.New("Unable to list paths from etcd backend")
	} else if resp.Node == nil {
		return nil, errors.New("Unable to list paths from etcd backend")
	} else if len(resp.Node.Nodes) == 0 {
		//list = append(list, string(resp.Node.Value))
	} else {
		for _, node := range resp.Node.Nodes {
			/* step: unmarshal the json data */
			document := &EtcdServiceDocument{}
			if err := json.Unmarshal([]byte(node.Value), document); err != nil {
				glog.Errorf("Unable to decode the service docoument: %s", node.Value)
				continue
			}
			/* step: validate the document */
			endpoint, err := IsValidServiceDocument(document)
			if err != nil {
				glog.Errorf(err.Error())
				continue
			}
			list = append(list, endpoint)
		}
	}
	return list, nil
}

func (etcd *EtcdDiscoveryService) Watch(si *services.Service) {
	if resp, err := etcd.client.Watch(si.Name, etcd.waitIndex, true, nil, nil); err != nil {
		glog.Error("etcd:", err)
	} else {
		etcd.waitIndex = resp.EtcdIndex + 1
	}
}

func IsValidServiceDocument(document *EtcdServiceDocument) (endpoint services.Endpoint, err error) {
	if document.IPaddress == "" {
		return endpoint, errors.New("Invalid service registration, document does not contain a ipaddress")
	}
	if document.Port == 0 {
		return endpoint, errors.New("Invalid service registration, document does not contain a port")
	}
	return services.Endpoint(fmt.Sprintf("%s:%d", document.IPaddress, document.Port)), nil
}
