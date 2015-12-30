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
	"os/signal"
	"runtime"
	"syscall"

	"github.com/gambol99/embassy/proxy"
	"github.com/gambol99/embassy/store"

	"github.com/golang/glog"
)

func main() {
	flag.Parse()
	/* step: set max processors */
	runtime.GOMAXPROCS(runtime.NumCPU())

	glog.Infof("Starting %s Service Proxy (%s), version: %s", name, author, version)

	/* step: create the services store */
	if services, err := store.NewServiceStore(); err != nil {
		glog.Fatalf("Failed to create the service provider, error: %s", err)
	} else {
		/* step: create the proxy service */
		if service, err := proxy.NewProxyService(services); err != nil {
			glog.Fatalf("Failed to create the proxy service, error: %s", err)
		} else {
			go func() {
				if err := service.ProxyConnections(); err != nil {
					glog.Fatalf("Failed to start the proxy service, error: %s", err)
				}
				os.Exit(1)
			}()
		}

		/* step: setup the channel for shutdown signals */
		signalChannel := make(chan os.Signal)
		signal.Notify(signalChannel, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
		/* step: wait on the signal */
		<-signalChannel
	}
}
