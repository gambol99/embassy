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
package main

import (
	"errors"
	"flag"
	"runtime/debug"

	"github.com/gambol99/embassy/config"
	"github.com/gambol99/embassy/proxy"
	"github.com/gambol99/embassy/services"
	"github.com/golang/glog"
	"runtime"
)

var proxies = make(map[services.ServiceID]proxy.ProxyService)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	/* step: parse command line options */
	configuration := ParseOptions()
	/* step: create a backend service provider */
	store, channel := LoadServicesStore(configuration)
	glog.Infof("Starting the Embassy Proxy Service, local ip: %s, hostname: %s", configuration.IPAddress, configuration.HostName)

	var _ = store
	for {
		request := <-channel
		glog.V(2).Infof("Received a backend service request: %s", request)
		if proxier := FindProxyService(request); proxier == nil {
			proxier, err := CreateServiceProxy(configuration, request)
			if err != nil {
				glog.Errorf("Unable to create the proxy service for: %s", request)
				continue
			}
			glog.Infof("Proxy Service: %s", proxier)
		} else {
			glog.Infof("Service request already being handled, skipping request")
		}
	}
}

func FindProxyService(si services.Service) proxy.ProxyService {
	glog.V(4).Infof("Looking up proxy for service: %s", si)
	proxier, found := proxies[si.ID]
	if !found {
		glog.V(5).Infof("A proxy for service: %s does not exist at present", si)
	}
	return proxier
}

func CreateServiceProxy(cfg *config.Configuration, si services.Service) (proxier proxy.ProxyService, err error) {
	defer func() {
		if recover := recover(); recover != nil {
			glog.Errorf("Failed to create a proxy service, error: %s, stack: %s", recover, debug.Stack())
			proxier = nil
			err = errors.New("Unable to create the proxy service")
		}
	}()
	glog.Infof("Service request: %s creating new proxy service as handler", si)
	proxier, err = proxy.NewProxyService(cfg, si)
	if err != nil {
		glog.Errorf("Failed to create proxy service for %s", si)
	}
	proxies[si.ID] = proxier
	proxier.StartServiceProxy()
	return proxier, err
}

func ParseOptions() *config.Configuration {
	flag.Parse()
	configuration := config.NewConfiguration()
	glog.Infof("Loading the configuration: %v", configuration)
	/* step: validate the service configuration */
	err := configuration.ValidConfiguration()
	Assert(err, "Invalid service configuration options, please check usage")
	return configuration
}

func LoadServicesStore(cfg *config.Configuration) (services.ServiceStore, services.ServiceStoreChannel) {
	glog.V(5).Infof("Attempting to create a new services store")
	channel := make(services.ServiceStoreChannel, 10)
	store := services.NewServiceStore(cfg)
	/* step: add the docker provider */
	services.AddDockerServiceStore(store, cfg)
	/* step: add ourselves as a listener */
	store.AddServiceListener(channel)
	/* step: start the discovery process */
	Assert(store.FindServices(), "Unable to start the discovery services")
	return store, channel
}

func Assert(err error, message string) {
	if err != nil {
		glog.Fatalf("%s, error: %s", message, err)
	}
}
