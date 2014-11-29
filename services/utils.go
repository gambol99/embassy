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
	"github.com/gambol99/embassy/utils"
	"github.com/gambol99/embassy/config"
	"github.com/golang/glog"
)

func NewServiceStore(config *config.Configuration) ServiceStore {
	/* step: has the backend been hardcoded on the command line, if so we use a fixed backend service */
	return &ServiceStoreImpl{config,
		make(BackendServiceChannel, 5),      	// channel to pass to providers
		make(map[string]ServiceProvider, 0), 	// a map of providers
		make([]ServiceStoreChannel, 0),			// a list of people listening for service updates
		make(utils.ShutdownSignalChannel)}
}

func AddDockerServiceStore(store ServiceStore, cfg *config.Configuration) error {
	docker_store, err := NewDockerServiceStore(cfg)
	if err != nil {
		glog.Errorf("Unable to create the docker service store, error: %s", err)
		return err
	}
	store.AddServiceProvider("docker", docker_store)
	return nil
}
