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
	"errors"
	"unsafe"
	"sync/atomic"

	"github.com/gambol99/embassy/proxy/services"
	"github.com/gambol99/embassy/utils"
	"github.com/golang/glog"
)

/* an endpoint is ip:port */
type Endpoint string

/* a channel used by the provider to say something has changed */
type EndpointEventChannel chan EndpointEvent

type EndpointsStore interface {
	/* shutdown the discovery agent */
	Close()
	/* retrieve a list of the current endpoints */
	ListEndpoints() ([]Endpoint, error)
	/* keep and eye on the endpoints report back on changes */
	WatchEndpoints()
	/* synchronize the endpoints for this service */
	Synchronize() error
	/* add an listener to discovery events */
	AddEventListener(EndpointEventChannel)
}

type EndpointsStoreService struct {
	/* the service agent is running for */
	Service services.Service
	/* the backend provider - etcd | consul | something else */
	Provider EndpointsProvider
	/* the current list of endpoints for this service */
	Endpoints unsafe.Pointer
	/* channel for listeners on endpoints */
	Listeners []EndpointEventChannel
	/* channel for shutdown signal */
	Shutdown utils.ShutdownSignalChannel
}

func (r *EndpointsStoreService) AddEventListener(channel EndpointEventChannel) {
	glog.V(5).Infof("Adding listener for endpoint events, channel: %V", channel)
	r.Listeners = append(r.Listeners, channel)
}

func (ds *EndpointsStoreService) ListEndpoints() ([]Endpoint,error) {
	/* step: pull a list of paths from the backend */
	return *(*[]Endpoint)(atomic.LoadPointer(&ds.Endpoints)), nil
}

func (ds *EndpointsStoreService) PushEventToListeners(event EndpointEvent) {
	glog.V(5).Infof("Pushing the event: %s to all listeners", event)

	/* create the event for us and wrap the service */
	event.Service = ds.Service

	/* step: send the event to all the listeners */
	for _, listener := range ds.Listeners {
		/* step: we run this in a go-routine not to block */
		go func() {
			glog.V(12).Infof("Pushing the event: %s to listener: %v", event, listener)
			listener <- event
		}()
	}
}

func (ds EndpointsStoreService) Close() {
	glog.Infof("Shutting down the endpoints store for service: %s", ds.Service)
	ds.Shutdown <- true
}

func (ds *EndpointsStoreService) Synchronize() error {
	glog.V(3).Infof("Synchronize the endpoints for service: %s", ds.Service)
	endpoints, err := ds.Provider.List(&ds.Service)
	if err != nil {
		glog.Errorf("Attempt to resynchronize the endpoints failed for service: %s, error: %s", ds.Service, err)
		return errors.New("Failed to resync the endpoints")
	}
	glog.V(3).Infof("Service: %s, endpoints: %s", ds.Service, endpoints)
	/* step: we register any new endpoints - using the endpoint id as key into the map */
	atomic.StorePointer(&ds.Endpoints,unsafe.Pointer(&endpoints))
	return nil
}

/* Go-routine listens to events from the store provider and passes them up the chain to listened (namely the proxy */
func (ds *EndpointsStoreService) WatchEndpoints() {
	glog.V(3).Infof("Watching for changes on service: %s", ds.Service)
	go func() {
		/* Never say die ! unless they tell us to */
		for {
			glog.V(4).Infof("Starting to watch endpoints for service: %s, path: %s", ds.Service, ds.Service.Name)
			watchChannel, err := ds.Provider.Watch(&ds.Service)
			if err != nil {
				glog.Errorf("Unable to start the watcher for service: %s, error: %s", ds.Service, err)
				return
			}
			/* step: we simply wait for updates from the watcher or an kill switch */
			for {
				select {
				case update := <-watchChannel:
					glog.V(4).Infof("Endpoints has changed for service: %s, updating the endpoints", ds.Service)
					/* step: update our endpoints */
					ds.Synchronize()
					/* step: push the event to the listeners */
					ds.PushEventToListeners(update)
				case <-ds.Shutdown:
					glog.Infof("Shutting down the provider for service: %s", ds.Service)
					/* step: push downstream the kill signal to provider */
					ds.Provider.Close()
					return
				}
			}
		}
	}()
}
