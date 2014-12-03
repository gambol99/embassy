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
	"github.com/gambol99/embassy/utils"
	"github.com/gambol99/embassy/proxy/services"
	"github.com/golang/glog"
)

func NewServiceStore() ServiceStore {
	return &ServiceStoreImpl{
		// channel to pass to providers
		make(BackendServiceChannel, 5),
		// a map of providers
		make(map[string]ServiceProvider, 0),
		// a list of people listening for service updates
		make([]services.ServiceEventsChannel, 0),
		// shutdown signal for the service
		make(utils.ShutdownSignalChannel)}
}

func AddDockerServiceStore(store ServiceStore) error {
	docker_store, err := NewDockerServiceStore()
	if err != nil {
		glog.Errorf("Unable to create the docker service store, error: %s", err)
		return err
	}
	store.AddServiceProvider("docker", docker_store)
	return nil
}
