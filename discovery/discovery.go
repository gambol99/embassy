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
	"sync"
	"time"

	"github.com/gambol99/embassy/config"
	"github.com/gambol99/embassy/services"
	"github.com/golang/glog"
)

const (
	DISCOVERY_STORES = `^(consul|etcd):\/\/`
)

type DiscoveryStoreChannel chan services.Service

type DiscoveryStore interface {
	ShutdownDiscovery() error
	ListEndpoints() ([]services.Endpoint, error)
	WatchEndpoints(channel DiscoveryStoreChannel)
	Synchronize() error
}

type DiscoveryStoreService struct {
	sync.RWMutex
	Service   services.Service
	Store     DiscoveryStoreProvider
	Config    *config.Configuration
	Endpoints []services.Endpoint
}

type DiscoveryStoreProvider interface {
	List(*services.Service) ([]services.Endpoint, error)
	Watch(*services.Service) error
}

func NewDiscoveryService(cfg *config.Configuration, si services.Service) (DiscoveryStore, error) {
	glog.Infof("Creating a new discovery agent for service: %s", si)
	/* step: check if the store provider is supported */
	if !IsDiscoveryStore(cfg.DiscoveryURI) {
		return nil, errors.New("The backend discovery store specified is not supported")
	}
	var provider DiscoveryStoreProvider
	var err error
	discovery := new(DiscoveryStoreService)
	discovery.Service = si
	discovery.Config = cfg
	discovery.Endpoints = make([]services.Endpoint, 0)

	/* step: create the backend provioder */
	switch cfg.GetDiscoveryURI().Scheme {
	case "etcd":
		glog.Infof("Using Etcd as discovery backend, uri: %s", cfg.DiscoveryURI)
		provider, err = NewEtcdStore(cfg)
	case "consul":
		glog.Infof("Using Consul as discovery backend, uri: %s", cfg.DiscoveryURI)
		provider, err = NewConsulStore(cfg)
	default:
		glog.Errorf("The discovery backend %s is not supported, please check usage", cfg.DiscoveryURI)
		return nil, errors.New("Invalid discovery backend")
	}
	if err != nil {
		glog.Errorf("Unable to initialize the Etcd backend store, error: %s", err)
		return nil, err
	}
	discovery.Store = provider
	return discovery, nil
}

func (ds *DiscoveryStoreService) ListEndpoints() (endpoints []services.Endpoint, err error) {
	/* step: pull a list of paths from the backend */
	ds.RLock()
	defer ds.RUnlock()
	return ds.Endpoints, nil
}

func (ds DiscoveryStoreService) ShutdownDiscovery() error {

	return nil
}

func (ds *DiscoveryStoreService) Synchronize() error {
	glog.V(3).Infof("Resynchronizing the endpoints for service: %s", ds.Service)
	ds.Lock()
	defer ds.Unlock()
	endpoints, err := ds.Store.List(&ds.Service)
	if err != nil {
		glog.Errorf("Attempt to resynchronize the endpoints failed for service: %s, error: %s", ds.Service, err)
		return errors.New("Failed to resync the endpoints")
	}
	/* step: we register any new endpoints - using the endpoint id as key into the map */
	ds.Endpoints = endpoints
	glog.V(4).Infof("Updating the endpoints for service: %s", ds.Service)
	return nil
}

func (ds *DiscoveryStoreService) WatchEndpoints(channel DiscoveryStoreChannel) {
	glog.V(2).Info("Watching service: %s", ds.Service)
	go func(ds *DiscoveryStoreService) {
		for {
			glog.V(4).Infof("Waiting for endpoints on service: %s to change", ds.Service)
			/* step: block and wait for something, anything to change */
			if err := ds.Store.Watch(&ds.Service); err != nil {
				time.Sleep(5 * time.Second )
				continue
			}
			glog.V(4).Infof("Endpoints has changed for service: %s", ds.Service)
			/* step: pull an updated list of the endpoints */
			channel <- ds.Service
		}
	}(ds)
}

func IsDiscoveryStore(uri string) bool {
	var validator = regexp.MustCompile(DISCOVERY_STORES)
	if found := validator.MatchString(uri); found {
		return true
	}
	glog.Errorf("The backend: %s is not supported, please check usage", uri)
	return false
}
