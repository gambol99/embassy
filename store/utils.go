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

	"github.com/gambol99/embassy/config"
	"github.com/gambol99/embassy/proxy/services"

	"github.com/golang/glog"
)

const (
	DOCKER_STORE = "docker"
	STATIC_STORE = "static"
)

func NewServiceStore() (services.ServiceStore, error) {
	glog.Infof("Initializing the services provider: %s", config.Options.Provider)
	var store services.ServiceStore
	var err error
	switch config.Options.Provider {
	case DOCKER_STORE:
		store, err = NewDockerServiceStore()
	case STATIC_STORE:
		store, err = NewStaticServiceStore()
	default:
		glog.Errorf("The services provider: %s is not supported, please check documentation")
		return nil, errors.New("The service provider: " + config.Options.Provider + " is not supported")
	}
	return store, err
}

// Converts the backend definition into an event and push to those whom are listening
// Params:
//		channel:		the channel we are pushing the events to
// 		definitions:	a slice of definitions to push
//		operation:		adding or removing
func pushServices(ch services.ServicesChannel, definitions []Definition, adding bool) {
	// Iterate and convert the backend definitions into services
	for _, definition := range definitions {
		// Attempt to convert the definition to a service request
		service, err := definition.GetService()
		if err != nil {
			glog.Errorf("Failed to convert the definition: %s to a service, error: %s", definition, err)
			continue
		}

		var event services.ServiceEvent
		event.Service = service
		event.Action = services.SERVICE_REMOVAL
		if adding {
			event.Action = services.SERVICE_REQUEST
		}

		// We perform this in a go-routine not to allow a receiver from blocking us
		go func() {
			ch <- event
		}()
	}
}
