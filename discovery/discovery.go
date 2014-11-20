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
	"errors"
	"regexp"

	"github.com/gambol99/embassy/config"
	"github.com/gambol99/embassy/services"
	"github.com/golang/glog"
)

const (
	DISCOVERY_STORES = `^(consul|etcd):\/\/`
)

type DiscoveryEvent struct {
	Endpoint services.Endpoint
}

type DiscoveryStoreChannel chan DiscoveryEvent

type DiscoveryStore interface {
	ListEndpoints() ([]services.Endpoint, error)
	WatchEndpoints(channel DiscoveryStoreChannel)
}

type DiscoveryStoreService struct {
	Service services.Service
	Store   DiscoveryStoreProvider
	Config  *config.Configuration
}

type DiscoveryStoreProvider interface {
	List(*services.Service) ([]services.Endpoint, error)
	Watch(*services.Service)
}

func NewDiscoveryService(config *config.Configuration, service services.Service) (DiscoveryStore, error) {
	glog.Infof("Creating a new discovery agent for service: %s", service)
	/* step: check if the store provider is supported */
	if !IsDiscoveryStore(config.DiscoveryURI) {
		return nil, errors.New("The backend discovery store specified is not supported")
	}
	var provider DiscoveryStoreProvider
	var err error
	discovery := new(DiscoveryStoreService)
	discovery.Config = config
	switch config.DiscoveryURI {
	case "etcd":
		glog.Infof("Using Etcd as discovery backend, uri: %s", config.DiscoveryURI)
		provider, err = NewEtcdStore(config)
	case "consul":
		glog.Infof("Using Consul as discovery backend, uri: %s", config.DiscoveryURI)
		provider, err = NewConsulStore(config)
	}
	if err != nil {
		glog.Errorf("Unable to initialize the Etcd backend store, error: %s", err)
		return nil, err
	}
	discovery.Store = provider
	return discovery, nil
}

func (ds DiscoveryStoreService) ListEndpoints() ([]services.Endpoint, error) {
	/* step: pull a list of paths from the backend */
	endpoints, err := ds.Store.List(&ds.Service)
	if err != nil {
		glog.Errorf("Unable to retrieve endpoints from the store, error: %s", err)
		return nil, err
	}
	if len(endpoints) <= 0 {
		glog.V(2).Infof("We have no endpoints avaible for service: %s", ds.Service)
	}
	/* step: iterate and pull build the endpoints */
	return endpoints, nil
}

func (d DiscoveryStoreService) WatchEndpoints(channel DiscoveryStoreChannel) {
	glog.V(2).Info("Watching service: %s", d.Service)
	go func() {
		for {
			/* step: block and wait for something, anything to change */
			d.Store.Watch(&d.Service)
			/* step: pull an updated list of the endpoints
			if endpoints, err := d.ListEndpoints(); err != nil {
				glog.Error("Unable to retrieve an updated list of paths from factory, error:", err)
				continue
			}
			*/
		}
	}()
}

func IsDiscoveryStore(uri string) bool {
	var validator = regexp.MustCompile(DISCOVERY_STORES)
	if found := validator.MatchString(uri); found {
		return true
	}
	glog.Errorf("The backend: %s is not supported, please check usage", uri)
	return false
}
