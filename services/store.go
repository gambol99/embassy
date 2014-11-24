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

package services

import (
	"github.com/golang/glog"
	"errors"
	"github.com/gambol99/embassy/config"
)

type ServiceStoreChannel chan Service
type BackendServiceChannel chan BackendDefinition

type ServiceStore interface {
	ShutdownStore() error
	FindServices() error
	AddServiceListener(ServiceStoreChannel)
	AddServiceProvider(name string, provider ServiceProvider) error
}

type ServiceProvider interface {
	StreamServices(BackendServiceChannel) error
}

type ServiceStoreImpl struct {
	Config *config.Configuration
	Channel BackendServiceChannel
	Providers map[string]ServiceProvider
	Listeners []ServiceStoreChannel
}

func NewServiceStore(config *config.Configuration) (ServiceStore) {
	/* step: has the backend been hardcoded on the command line, if so we use a fixed backend service */
	glog.V(5).Infof("Creating services store, configuration: %V", config )
	return &ServiceStoreImpl{ config,
		make(BackendServiceChannel, 5),			// channel to pass to providers
		make(map[string]ServiceProvider,0),		// a map of providers
		make([]ServiceStoreChannel,0)}			// a list of people listening for service updates
}

func (r *ServiceStoreImpl) AddServiceListener(channel ServiceStoreChannel) {
	glog.V(2).Infof("Adding a new service listener to the ServiceStore, channel: %V", channel )
	r.Listeners = append(r.Listeners,channel)
}

func (r *ServiceStoreImpl) AddServiceProvider(name string, provider ServiceProvider) error {
	if _, found := r.Providers[name]; found {
		glog.Errorf("The provider: %s is already registered", name )
		return errors.New("The procider is already registered")
	}
	glog.Infof("Adding Service Provider: %s added to providers", name )
	r.Providers[name] = provider
	return nil
}

func (r *ServiceStoreImpl) FindServices() error {
	if len(r.Providers) <= 0 {
		return errors.New("You have not registered any providers")
	}
	glog.V(3).Infof("Starting the discovery of services loop, providers: %d", len(r.Providers) )
	/* step: start the providers service stream */
	for name, provider := range r.Providers {
		glog.Infof("Starting the service stream from provider: %s", name )
		if err := provider.StreamServices(r.Channel); err != nil {
			glog.Errorf("Unable to start provider: %s service stream, error: %s", name, err )
		}
	}
	go func() {
		for {
			/* step: wait for a backend definition to be channeled from a provider */
			definition := <-r.Channel
			glog.V(5).Infof("We have recieved a definition request from a provider, definition: %s", definition )
			/* step: convert the definition into a service */
			service, err := definition.GetService()
			if err != nil {
				glog.Errorf("The service definition is invalid, error: %s", err)
				continue
			}
			glog.V(5).Infof("Sending the service on to event listeners (%d)", len(r.Listeners) )
			/* step: send the service to everyone that is listening */
			for _, channel := range r.Listeners {
				go func(c ServiceStoreChannel) {
					channel <- service
					glog.V(4).Infof("Sent the service to listener")
				}(channel)
			}
		}
	}()
	return nil
}

func (r *ServiceStoreImpl) ShutdownStore() error {
	return nil
}


