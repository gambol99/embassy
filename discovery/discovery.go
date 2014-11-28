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

import (
	"errors"
	"regexp"
	"sync"

	"github.com/gambol99/embassy/config"
	"github.com/gambol99/embassy/services"
	"github.com/golang/glog"
)

const (
	DISCOVERY_STORES = `^(consul|etcd):\/\/`
)

type DiscoveryEventsChannel chan EndPoint
type EndpointUpdateChannel chan EndPoint

type DiscoveryStore interface {
	/* shutdown the discovery agent */
	Close() error
	/* retrieve a list of the current endpoints */
	ListEndpoints() ([]services.Endpoint, error)
	/* keep and eye on the endpoints report back on changes */
	WatchEndpoints()
	/* synchronize the endpoints for this service */
	Synchronize() error
	/* add an listener to discovery events */
	AddEventListener(DiscoveryEventsChannel)
}

type DiscoveryStoreService struct {
	/* locking used to control access to the endpoints */
	sync.RWMutex
	/* the service agent is running for */
	Service services.Service
	/* the backend provider - etcd | consul */
	Provider DiscoveryStoreProvider
	/* the service configuration */
	Config *config.Configuration
	/* the current list of endpoints for this service */
	Endpoints []services.Endpoint
	/* channel for listeners on endpoints */
	Listeners []DiscoveryEventsChannel
	/* channel for shutdown signal */
	Shutdown chan bool
}

type DiscoveryStoreProvider interface {
	/* get a list of the endpoints from the backend */
	List(*services.Service) ([]services.Endpoint, error)
	/* watch for changes on the backend */
	Watch(*services.Service) (EndpointUpdateChannel,error)
	/* shutdown and clean up the provider */
	Close()
}

func NewDiscoveryService(cfg *config.Configuration, si services.Service) (DiscoveryStore, error) {
	/* step: check the cache first of all */
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

func (r *DiscoveryStoreService) AddEventListener(channel DiscoveryStoreChannel) {
	glog.V(5).Infof("Adding listener for discovery events, channel: %V", channel )
	r.Listeners = append(r.Listeners, channel)
}

func (ds *DiscoveryStoreService) ListEndpoints() (endpoints []services.Endpoint, err error) {
	/* step: pull a list of paths from the backend */
	ds.RLock()
	defer ds.RUnlock()
	return ds.Endpoints, nil
}

func (ds *DiscoveryStoreService) PushEventToListeners(event EndpointEvent) {	
	glog.V(3).Infof("Pushing the event: %s to all listeners", event )
	for _, listener := range ds.Listeners {
		/* step: we run this in a goroutine not to block */
		go func(ch DiscoveryEventsChannel) {
			glog.V(12).Infof("Pushing the event: %s to listener: %v", event, ch )
			ch <- event
		}(listener)
	}
}

func (ds DiscoveryStoreService) Close() error {

	return nil
}

func (ds *DiscoveryStoreService) Synchronize() error {
	glog.V(3).Infof("Synchronize the endpoints for service: %s", ds.Service)
	ds.Lock()
	defer ds.Unlock()
	endpoints, err := ds.Store.List(&ds.Service)
	if err != nil {
		glog.Errorf("Attempt to resynchronize the endpoints failed for service: %s, error: %s", ds.Service, err)
		return errors.New("Failed to resync the endpoints")
	}
	/* step: we register any new endpoints - using the endpoint id as key into the map */
	ds.Endpoints = endpoints
	glog.V(5).Infof("Updating the endpoints for service: %s, endpoints: %s", ds.Service, ds.Endpoints)
	return nil
}

/* Goroutine listens to events from the store provider and passes them up the chain to listened (namely the proxy */
func (r *DiscoveryStoreService) WatchEndpoints() error {
	glog.V(3).Info("Watching for changes on service: %s", r.Service)
	go func() {
		glog.V(4).Infof("Starting to watch endpoints for service: %s, path: %s", r.Service, r.Service.Name )
		watchChannel, err := r.Provider.Watch(&r.Service)
		if err != nil {
			glog.Errorf("Unable to start the watcher for service: %s, error: %s", r.Service, err )
			return
		}
		/* step: we simply wait for updates from the watcher */
		for {
			switch {
			case update := <-watchChannel:
				glog.V(4).Infof("Endpoints has changed for service: %s, updating the endpoints", ds.Service)
				/* step: push the event to the listeners */
			for _, listener := range r.Listeners {
				go func(client DiscoveryEventsChannel) {
					glog.V(5).Infof("Pushing the discovery event to listener channel: %V", client)
					client <- event
				}(listener)
			}
			case signal := <-r.Shutdown:
				/* step: we've been requested to shutdown :-( */
				glog.Infof("Shutting down the watch service on service: %s", r.Service)
				/* step: signal the provider */
				r.Provider.Close()
				return
			}
		}
	}()
	return nil
}

func IsDiscoveryStore(uri string) bool {
	var validator = regexp.MustCompile(DISCOVERY_STORES)
	if found := validator.MatchString(uri); found {
		return true
	}
	glog.Errorf("The backend: %s is not supported, please check usage", uri)
	return false
}
