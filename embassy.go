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

	configuration := config.NewServiceConfiguration()
	/* step: validate the service configuration */
	if err := configuration.ValidConfiguration(); err != nil {
		glog.Fatalf("Invalid service configuration, error: %s", configuration.ValidConfiguration())
	}

	/* step: create a backend service provider */
	serviceUpdates := make(services.ServiceStoreChannel, 3)
	if _, err := services.NewServiceStore(configuration, serviceUpdates); err != nil {
		glog.Fatalf("Unable to create the backend request service, error: %s", err)
	} else {
		/* step: we listen out for backend service requests and create a proxy on them */
		for {
			backendRequest := <-serviceUpdates
			glog.V(2).Info("%s: received a backend service request: %s", config.ProgName(), backendRequest)
			/* step: check if this is a duplicate request */
			if proxy, found := ProxyServiceLookup(backendRequest); found {
				glog.Errorf("%s: backend service request invalid, error: %s", config.ProgName(), err)
				continue
			} else {
				glog.Errorf("%s: we need to create new proxy for service: %s", config.ProgName(), backendRequest)
				var _ = proxy
			}
		}
	}
}
