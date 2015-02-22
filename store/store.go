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

package store

import (
	"errors"
	"flag"

	"github.com/gambol99/embassy/proxy/services"
	"github.com/gambol99/embassy/utils"
	"github.com/golang/glog"
)

const (
	/* the default services provider */
	DEFAULT_STORE_PROVIDER = "docker"
)

var (
	provider *string
)

func init() {
	provider = flag.String("provider", DEFAULT_STORE_PROVIDER, "the services provider to use, either docker or static")
}

type ServiceStore interface {
	services.ServiceStore
	AddServiceProvider(name string, provider ServiceProvider) error
}

/*
the channel is used to send events from the backends to the service store and
then upstream to the proxy service
*/
type BackendServiceChannel chan DefinitionEvent

/*
The service provider reads in service request from the containers and push them
up stream to the ServiceStore
*/
type ServiceProvider interface {
	StreamServices(BackendServiceChannel) error
}

/*
the implementation for the services store
 - the service configuration
 - a channel for providers to send request
 - a list of providers
 - a collection of people listening to events
 - a shutdown down signal
*/
type ServiceStoreImpl struct {
	BackendChannel BackendServiceChannel
	Providers      map[string]ServiceProvider
	Listeners      []services.ServiceEventsChannel
	Shutdown       utils.ShutdownSignalChannel
}

func NewServiceStore() (ServiceStore, error) {
	glog.Infof("Initializing the services provider: %s", *provider)

	/* step: create the services store */
	service := &ServiceStoreImpl{
		// channel to pass to providers
		make(BackendServiceChannel, 20),
		// a map of providers
		make(map[string]ServiceProvider, 0),
		// a list of people listening for service updates
		make([]services.ServiceEventsChannel, 0),
		// shutdown signal for the service
		make(utils.ShutdownSignalChannel)}

	/* step: create and register the service provider */
	switch *provider {
	case "docker":
		glog.Infof("Registering the docker services provider")
		/* step: we use the default docker store for now */
		if agent, err := AddDockerServiceStore(); err != nil {
			glog.Errorf("Failed to add the docker service provider, error: %s", err)
			return nil, err
		} else {
			service.AddServiceProvider(*provider, agent)
		}
	case "static":
		glog.Infof("Registering the static services provider")
		/* step: create the static services store */
		if agent, err := AddStaticServiceProvider(); err != nil {
			glog.Errorf("Failed to add the docker service provider, error: %s", err)
			return nil, err
		} else {
			service.AddServiceProvider(*provider, agent)
		}
	default:
		glog.Errorf("The services provider: %s is not supported, please check documentation")
		return nil, errors.New("The service provider: " + *provider + " is not supported")
	}

	return service, nil
}

func (r *ServiceStoreImpl) AddServiceListener(channel services.ServiceEventsChannel) {
	glog.V(2).Infof("Adding a new service listener to the ServiceStore, channel: %V", channel)
	r.Listeners = append(r.Listeners, channel)
}

func (r *ServiceStoreImpl) AddServiceProvider(name string, provider ServiceProvider) error {
	if _, found := r.Providers[name]; found {
		glog.Errorf("The provider: %s is already registered", name)
		return errors.New("The procider is already registered")
	}
	glog.Infof("Adding Service Provider: %s added to providers", name)
	r.Providers[name] = provider
	return nil
}

func (r *ServiceStoreImpl) PushServiceEvent(service services.ServiceEvent) {
	go func() {
		for _, channel := range r.Listeners {
			glog.V(6).Infof("Pushed the service event: %s to listener: %V", service, channel)
			channel <- service
		}
	}()
}

func (r *ServiceStoreImpl) Start() error {
	if len(r.Providers) <= 0 {
		return errors.New("You have not registered any service providers")
	}
	glog.V(3).Infof("Starting the services loop, providers: %d", len(r.Providers))
	/* step: start the providers service stream */
	for name, provider := range r.Providers {
		glog.Infof("Starting the service stream from provider: %s", name)
		if err := provider.StreamServices(r.BackendChannel); err != nil {
			glog.Errorf("Unable to start provider: %s service stream, error: %s", name, err)
		}
	}
	r.FindServices()
	return nil
}

func (r *ServiceStoreImpl) FindServices() error {
	glog.V(4).Infof("Entered the service stream, pushing service across channel")
	go func() {
		for {
			select {
			case <-r.Shutdown:
				/* step: shutdown the service store */
				glog.Infof("Shutting down the Services Store")
			case definition := <-r.BackendChannel:
				glog.V(5).Infof("Recieved definition from provider, definition: %s", definition)
				// step: convert the definition into a service
				if service, err := definition.GetService(); err != nil {
					glog.Errorf("The service definition is invalid, error: %s", err)
				} else {
					var event services.ServiceEvent
					event.Service = service
					switch definition.Operation {
					case DEFINITION_SERVICE_ADDED:
						event.Action = services.SERVICE_REQUEST
					case DEFINITION_SERVICE_REMOVED:
						event.Action = services.SERVICE_REMOVAL
					default:
						glog.Errorf("Unable definition operation: %d", definition.Operation)
						continue
					}
					r.PushServiceEvent(event)
				}
			}
		}
	}()
	return nil
}

/* Creates a signal to shutdown any resources the ServiceStore is holding */
func (r *ServiceStoreImpl) Close() {
	glog.Infof("Attempting to shutdown the Services Store")
	r.Shutdown <- true
}
