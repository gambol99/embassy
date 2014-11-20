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

import (
	"github.com/armon/consul-api"
	"github.com/gambol99/embassy/config"
	"github.com/gambol99/embassy/services"
	//"github.com/golang/glog"
)

type ConsulStore struct {
	client    *consulapi.Client
	waitIndex uint64
}

func NewConsulStore(cfg *config.Configuration) (DiscoveryStoreProvider, error) {
	consul := consulapi.DefaultConfig()
	consul.Address = cfg.GetDiscoveryURI().Host
	client, err := consulapi.NewClient(consul)
	if err != nil {
		return nil, err
	}
	return &ConsulStore{client, 0}, nil
}

func (cs ConsulStore) List(si *services.Service) ([]services.Endpoint, error) {
	/*
		if kv, _, err := cs.client.KV().List(path, &consulapi.QueryOptions{}); err != nil {
			glog.Errorf("Consul error: %s", err)
			return []string{}, nil
		} else {
			list := make([]string, 0)
			for _, pair := range kv {
				list = append(list, string(pair.Value))
			}
			return list, nil
		}
	*/
	return nil, nil
}

func (cs *ConsulStore) Watch(si *services.Service) {
	/*
		if _, meta, err := cs.client.KV().Get(path, &consulapi.QueryOptions{WaitIndex: s.waitIndex}); err != nil {
			glog.Errorf("Consul error: %s", err)
			return err
		} else {
			s.waitIndex = meta.LastIndex
			return nil
		}
	*/
}
