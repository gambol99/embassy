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
	"flag"

	"github.com/gambol99/embassy/config"
	"github.com/gambol99/embassy/proxy"
	"github.com/gambol99/embassy/services"
	"github.com/golang/glog"
	"runtime"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	/* step: parse command line options */
	configuration := ParseOptions()

	/* step: create a backend service provider */
	store := LoadServicesStore(configuration)
	glog.Infof("Starting the Embassy Proxy Service, local ip: %s, hostname: %s", configuration.IPAddress, configuration.HostName)

	/* step: create the proxy service */
	proxy_store, err := proxy.NewProxyStore(configuration, store)
	if err != nil {
		glog.Errorf("Failed to create the proxy service, error: %s", err)
		return
	}

	/* kick of the proxy and wait */
	done := make(chan bool)
	go func() {
		if err := proxy_store.Start(); err != nil {
			glog.Errorf("Failed to start the proxy service, error: %s", err)
			return
		}
		done <- true
	}()
	var finished = <-done
	glog.Infof("Exitting the proxy service: %s", finished)
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

func LoadServicesStore(cfg *config.Configuration) services.ServiceStore {
	glog.V(5).Infof("Attempting to create a new services store")
	store := services.NewServiceStore(cfg)
	/* step: add the docker provider */
	services.AddDockerServiceStore(store, cfg)
	/* step: return the store */
	return store
}

func Assert(err error, message string) {
	if err != nil {
		glog.Fatalf("%s, error: %s", message, err)
	}
}
