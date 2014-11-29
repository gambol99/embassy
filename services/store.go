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
	"errors"

	"github.com/gambol99/embassy/utils"
	"github.com/gambol99/embassy/config"
	"github.com/golang/glog"
)

type ServiceStore interface {
	Close()
	FindServices() error
	AddServiceListener(ServiceStoreChannel)
	AddServiceProvider(name string, provider ServiceProvider) error
}

/*
the channel is used by the service store to send service requests and removal
from over to the proxy service
 */
type ServiceStoreChannel chan ServiceEvent

/*
the channel is used to send events from the backends to the service store and
then upstream to the proxy service
 */
type BackendServiceChannel chan Definition
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
	Config    *config.Configuration
	Channel   BackendServiceChannel
	Providers map[string]ServiceProvider
	Listeners []ServiceStoreChannel
	Shutdown  utils.ShutdownSignalChannel
}

func (r *ServiceStoreImpl) AddServiceListener(channel ServiceStoreChannel) {
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

func (r *ServiceStoreImpl) PushServiceEvent(service Service) {
	for _, channel := range r.Listeners {
		go func() {
			channel <- service
		}()
	}
}

func (r *ServiceStoreImpl) FindServices() error {
	if len(r.Providers) <= 0 {
		return errors.New("You have not registered any providers")
	}
	glog.V(3).Infof("Starting the discovery of services loop, providers: %d", len(r.Providers))
	/* step: start the providers service stream */
	for name, provider := range r.Providers {
		glog.Infof("Starting the service stream from provider: %s", name)
		if err := provider.StreamServices(r.Channel); err != nil {
			glog.Errorf("Unable to start provider: %s service stream, error: %s", name, err)
		}
	}
	go func() {
		var definition Definition
		for {
			switch {
			case definition := <- r.Channel:
				/* step: wait for a backend definition to be channeled from a provider */
				definition := <-r.Channel
				glog.V(5).Infof("We have recieved a definition request from a provider, definition: %s", definition)
				/* step: convert the definition into a service */
				service, err := definition.GetService()
				if err != nil {
					glog.Errorf("The service definition is invalid, error: %s", err)
					continue
				}
				glog.V(5).Infof("Sending the service on to event listeners (%d)", len(r.Listeners))
				/* step: send the service to everyone that is listening */
				r.PushServiceEvent(service)
			case <- r.Shutdown:
				/* step: shutdown the service store */
				glog.Infof("Shutting down the ServicesStore")

			}
		}
	}()
	return nil
}

/*
Creates a signal to shutdown any resources the ServiceStore is holding
 */
func (r *ServiceStoreImpl) Close()  {
	glog.Infof("Attempting to shutdown the Services Store")
	r.Shutdown <- true
}
