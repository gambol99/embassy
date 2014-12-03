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
	"github.com/gambol99/embassy/utils"
	"github.com/gambol99/embassy/proxy/services"
	"github.com/golang/glog"
)

func NewEndpointsService(discovery string, si services.Service) (EndpointsStore, error) {
	/* step: check the cache first of all */
	glog.Infof("Initializing endpoints store for service: %s", si )
	/* step: check if the store provider is supported */
	endpoints := new(EndpointsStoreService)
	endpoints.Service = si
	endpoints.Endpoints = make([]Endpoint, 0)
	endpoints.Listeners = make([]EndpointEventChannel,0)
	endpoints.Shutdown = make(utils.ShutdownSignalChannel)
	/* step: create the backend provider */
	glog.Infof("Using Etcd as discovery backend, uri: %s", discovery )
	provider, err := NewEtcdStore(discovery)
	if err != nil {
		glog.Errorf("Unable to initialize the Etcd backend store, error: %s", err)
		return nil, err
	}
	endpoints.Provider = provider
	return endpoints, nil
}
