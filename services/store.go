/*
Copyright 2014 Rohith Jayawaredene All rights reserved.

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

	"github.com/gambol99/embassy/config"
	"github.com/golang/glog"
)

/*
  SERVICE=<SERVICE_NAME>[TAGS,..];<PORT>;
  BACKEND=etcd://localhost:4001
  BACKEND_REDIS_MASTER=redis.master;PORT
  BACKEND_REDIS_MASTER=redis.master[%{ENVIRONMENT|prod},dc1,]
  BACKEND_REDIS_MASTER=/%{PREFIX|services}/%{ENVIRONMENT|prod}/redis/master/6379/*
*/

type ServiceStoreChannel chan Service

type ServiceStore interface {
	DiscoverServices() error
}

func NewServiceStore(config *config.ServiceConfiguration, channel ServiceStoreChannel) (ServiceStore, error) {
	/* step: has the backend been hardcoded on the command line, if so we use a fixed backend service */
	if config.IsFixedBackend() {
		glog.V(1).Infof("Using Fixed Backend service: %s", config.FixedBackend)
		return nil, errors.New("Fixed Backend service is not presently supported")
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
