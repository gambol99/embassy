/*
Copyright 2014 Rohith Jayawardene All rights reserved.

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
package main

import (
	"flag"

	"github.com/gambol99/embassy/config"
	"github.com/gambol99/embassy/proxy"
	"github.com/gambol99/embassy/services"
	"github.com/golang/glog"
)

func ProxyServiceLookup(request services.Service) (*proxy.ProxyService, bool) {

	return nil, false
}

func CreateProxy(request services.Service) (*proxy.ProxyService, error) {

	return nil, nil
}

func main() {
	flag.Parse()
	configuration := config.NewConfiguration()
	glog.Infof("Loading the configuration: %v", configuration)

	/* step: validate the service configuration */
	if err := configuration.ValidConfiguration(); err != nil {
		glog.Fatalf("Invalid service configuration, error: %s", configuration.ValidConfiguration())
	}

	/* step: create a backend service provider */
	channel := make(services.ServiceStoreChannel, 3)
	store, err := services.NewServiceStore(configuration, channel)
	if err != nil {
		glog.Fatalf("Unable to create the backend request service, error: %s", err)
	}
	/* step: start the discovery process */
	if err := store.DiscoverServices(); err != nil {
		glog.Fatalf("Unable to start the discovery services, error: %s", err)
	}
	glog.V(3).Infof("Starting the services event loop")
	for {
		service_request := <-channel
		glog.V(2).Info("%s: received a backend service request: %s", config.ProgName(), service_request)
		/* step: check if this is a duplicate request */
		if proxy, found := ProxyServiceLookup(service_request); found {
			glog.Errorf("%s: backend service request invalid, error: %s", config.ProgName(), err)
			continue
		} else {
			glog.Errorf("%s: we need to create new proxy for service: %s", config.ProgName(), service_request)
			var _ = proxy
			var _ = store
		}
	}
}
