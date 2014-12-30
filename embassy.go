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
	"os"
	"runtime"

	"github.com/gambol99/embassy/proxy"
	"github.com/gambol99/embassy/store"
	"github.com/golang/glog"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	flag.Parse()

	glog.Infof("Starting the Embassy Docker Service Proxy, version: %s", Version)
	/* step: create the services store */
	services := store.NewServiceStore()
	/* step: we use the default docker store for now */
	if err := store.AddDockerServiceStore(services); err != nil {
		glog.Errorf("Failed to add the docker service provider, error: %s", err)
		os.Exit(1)
	}
	/* step: create the proxy service */
	service, err := proxy.NewProxyService(services)
	if err != nil {
		glog.Errorf("Failed to create the proxy service, error: %s", err)
		return
	}

	done := make(chan bool)
	go func() {
		if err := service.Start(); err != nil {
			glog.Errorf("Failed to start the proxy service, error: %s", err)
			return
		}
		done <- true
	}()
	<-done
	glog.Infof("Exitting the proxy service ")
}
