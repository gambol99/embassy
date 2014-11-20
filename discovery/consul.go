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

/*
#
#   Author: Rohith
#   Date: 2014-11-14 22:36:56 +0000 (Fri, 14 Nov 2014)
#
#  vim:ts=2:sw=2:et
#
*/

/*
import (
	//"net/url"

	"github.com/armon/consul-api"
	"github.com/golang/glog"
)

type ConsulDiscoveryService struct {
	client    *consulapi.Client
	waitIndex uint64
}

func NewConsulDiscoveryService(backend string, options map[string]string) (DiscoveryStoreProvider, error) {

		config := consulapi.DefaultConfig()
		if backend.Host != "" {
			config.Address = uri.Host
		}
		if client, err := consulapi.NewClient(config); err != nil {
			return nil, err
		} else {
			service := new(ConsulDiscoveryService)
			service.client = client
			return service, nil
		}

	return nil, nil
}

func (s ConsulDiscoveryService) List(path string) ([]string, error) {
	if kv, _, err := s.client.KV().List(path, &consulapi.QueryOptions{}); err != nil {
		glog.Errorf("Consul error: %s", err)
		return []string{}, nil
	} else {
		list := make([]string, 0)
		for _, pair := range kv {
			list = append(list, string(pair.Value))
		}
		return list, nil
	}
}

func (s ConsulDiscoveryService) Get(path string) string {
	if kv, _, err := s.client.KV().Get(path, &consulapi.QueryOptions{}); err != nil {
		glog.Errorf("Consul error: %s", err)
		return ""
	} else {
		if kv == nil {
			return ""
		}
		return string(kv.Value)
	}
}

func (s *ConsulDiscoveryService) Watch(path string) error {
	if _, meta, err := s.client.KV().Get(path, &consulapi.QueryOptions{WaitIndex: s.waitIndex}); err != nil {
		glog.Errorf("Consul error: %s", err)
		return err
	} else {
		s.waitIndex = meta.LastIndex
		return nil
	}
}
*/
