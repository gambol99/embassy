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
	"github.com/gambol99/embassy/config"
	"github.com/golang/glog"
)

type ServiceStoreChannel chan Service

type ServiceStore interface {
	DiscoverServices() error
}

func NewServiceStore(config *config.Configuration, channel ServiceStoreChannel) (ServiceStore, error) {
	/* step: has the backend been hardcoded on the command line, if so we use a fixed backend service */
	glog.V(5).Infof("Creating services store, configuration: %V", config)
	if config.FixedBackend != "" {
		glog.V(1).Infof("Using Fixed Backend service: %s", config.FixedBackend)
		if service, err := NewFixedServiceStore(config, channel); err != nil {
			glog.Fatalf("Unable to create the fixed backend service, error: %s", err)
		} else {
			return service, nil
		}
	} else {
		glog.V(1).Infof("Using Docker Backend service, socket: %s", config.DockerSocket)
		if service, err := NewDockerServiceStore(config, channel); err != nil {
			glog.Fatalf("Unable to create the docker backend service, error: %s", err)
		} else {
			return service, err
		}
	}
	return nil, nil
}