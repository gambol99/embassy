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
	"sync/atomic"
	"time"
	"unsafe"

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
	service services.Service
	/* the backend provider - etcd | consul | marathon */
	provider EndpointsProvider
	/* the current list of endpoints for this service */
	endpoints unsafe.Pointer
	/* channel for listeners on endpoints */
	listeners []EndpointEventChannel
	/* channel for shutdown signal */
	shutdown utils.ShutdownSignalChannel
}

func (r *EndpointsStoreService) AddEventListener(channel EndpointEventChannel) {
	glog.V(8).Infof("Adding listener for endpoint events, channel: %V", channel)
	r.listeners = append(r.listeners, channel)
}

func (ds *EndpointsStoreService) ListEndpoints() ([]Endpoint, error) {
	return *(*[]Endpoint)(atomic.LoadPointer(&ds.endpoints)), nil
}

func (ds EndpointsStoreService) Close() {
	glog.Infof("Shutting down the endpoints store for service: %s", ds.service)
	ds.shutdown <- true
}

func (ds *EndpointsStoreService) Synchronize() error {
	glog.V(5).Infof("Synchronize the endpoints for service: %s", ds.service)
	endpoints, err := ds.provider.List(&ds.service)
	if err != nil {
		glog.Errorf("Attempt to resynchronize the endpoints failed for service: %s, error: %s", ds.service, err)
		return errors.New("Failed to resync the endpoints")
	}
	glog.V(3).Infof("Service: %s, endpoints: %s", ds.service, endpoints)
	/* step: we register any new endpoints - using the endpoint id as key into the map */
	atomic.StorePointer(&ds.endpoints, unsafe.Pointer(&endpoints))
	return nil
}

/* Go-routine listens to events from the store provider and passes them up the chain to listened (namely the proxy */
func (ds *EndpointsStoreService) WatchEndpoints() {
	glog.V(3).Infof("Watching for changes on service: %s", ds.service)
	go func() {
		glog.V(3).Infof("Starting to watch endpoints for service: %s, path: %s", ds.service, ds.service.Name)
		watchChannel, err := ds.provider.Watch(&ds.service)
		if err != nil {
			glog.Errorf("Unable to start the watcher for service: %s, error: %s", ds.service, err)
			return
		}

		// A timer for debugging purposes, dump the endpoints
		timer  := time.NewTicker(10 * time.Second)
		resync := time.NewTicker(60 * time.Second)

		// step: we simply wait for updates from the watcher or an kill switch
		for {
			select {
			// We've received a change in the endpoints, lets resync
			case update := <-watchChannel:
				ds.Synchronize()
				ds.Upstream(update)

			case <-resync.C:
				ds.Synchronize()

			// Dump the endpoints for debugging purposes
			case <-timer.C:
				endpoints, _ := ds.ListEndpoints()
				glog.V(4).Infof("services: %s, endpoints: %s", ds.service, endpoints)

			// Shutdown the watcher
			case <-ds.shutdown:
				ds.provider.Close()
				return
			}
		}
	}()
}

func (ds *EndpointsStoreService) Upstream(event EndpointEvent) {
	glog.V(8).Infof("Pushing the event: %s to all listeners", event)

	/* create the event for us and wrap the service */
	event.Service = ds.service

	/* step: send the event to all the listeners */
	for _, listener := range ds.listeners {
		/* step: we run this in a go-routine not to block */
		go func() {
			glog.V(12).Infof("Pushing the event: %s to listener: %v", event, listener)
			listener <- event
		}()
	}
}

