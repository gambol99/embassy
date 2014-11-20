/*
Copyright 2014 Rohith Jayawaredene All rights reserved.

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

import (
	"github.com/coreos/go-etcd/etcd"
	"github.com/golang/glog"
)

type EtcdDiscoveryService struct {
	client    *etcd.Client
	waitIndex uint64
}

func NewEtcdDiscoveryService() (DiscoveryStoreProvider, error) {
	if uri, err := config.GetDiscoveryURI(); err != nil {
		glog.Errorf("Unable to create the etcd client, backend: %s, error: %s", uri, err)
		return nil, err
	} else {
		urls := make([]string, 0)
		if uri.Host != "" {
			urls = append(urls, "http://"+uri.Host)
		}
		return &EtcdDiscoveryService{client: etcd.NewClient(urls)}, nil
	}
}

func (etcd *EtcdDiscoveryService) List(service services.Service) ([]services.ServiceEndpoint, error) {
	list := make([]services.ServiceEndpoint, 0)
	/*
		if resp, err := etcd.client.Get(path, false, true); err != nil {
			glog.Error("etcd:", err)
			return
		}
		if resp.Node == nil {
			return
		}
		if len(resp.Node.Nodes) == 0 {
			list = append(list, string(resp.Node.Value))
		} else {
			for _, node := range resp.Node.Nodes {
				list = append(list, string(node.Value))
			}
		}
	*/
	return list, nil
}

func (etcd *EtcdDiscoveryService) Watch(service services.Service) {
	/*
		if resp, err := etcd.client.Watch(path, s.waitIndex, true, nil, nil); err != nil {
			glog.Error("etcd:", err)
		} else {
			s.waitIndex = resp.EtcdIndex + 1
		}
	*/
}
