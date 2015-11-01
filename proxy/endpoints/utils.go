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
	"net/url"
	"sync/atomic"
	"unsafe"

	"github.com/gambol99/embassy/proxy/services"
	"github.com/gambol99/embassy/utils"

	"github.com/golang/glog"
)

const (
	DISCOVERY_ETCD     = "etcd"
	DISCOVERY_CONSUL   = "consul"
	DISCOVERY_MARATHON = "marathon"
)

func NewEndpointsService(discovery string, si services.Service) (EndpointsStore, error) {
	/* step: check the cache first of all */
	glog.V(4).Infof("Initializing endpoints store for service: %s", si)
	// step: check if the store provider is supported
	endpoints := new(EndpointsStoreService)
	endpoints.service = si
	// Set to empty
	empty := make([]Endpoint, 0)
	atomic.StorePointer(&endpoints.endpoints, unsafe.Pointer(&empty))
	endpoints.listeners = make([]EndpointEventChannel, 0)
	endpoints.shutdown = make(utils.ShutdownSignalChannel)

	/* step: create the backend provider */

	uri, err := url.Parse(discovery)
	if err != nil {
		glog.Errorf("Failed to parse the discovery url: %s, error: %s", discovery, err)
		return nil, err
	}
	glog.V(3).Infof("Using endpoints agent: %s, discovery uri: %s", uri.Scheme, discovery)
	var provider EndpointsProvider
	switch uri.Scheme {
	case DISCOVERY_ETCD:
		provider, err = NewEtcdStore(discovery)
	case DISCOVERY_CONSUL:
		provider, err = NewConsulClient(discovery)
	case DISCOVERY_MARATHON:
		provider, err = NewMarathonClient(discovery)
	default:
		glog.Errorf("Failed to create endpoints agent, the backend: %s is not supported", discovery)
		return nil, errors.New("Unsupported backend " + discovery)
	}
	if err != nil {
		glog.Errorf("Failed to initialize the endpoint: %s, error: %s", discovery, err)
		return nil, err
	}
	glog.V(5).Infof("Succesfully initialize the endpoints %s", discovery)
	endpoints.provider = provider
	return endpoints, nil
}
