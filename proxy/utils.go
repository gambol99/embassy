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

package proxy

import (
	"net"
	"fmt"

	"github.com/gambol99/embassy/utils"
	"github.com/gambol99/embassy/proxy/services"
	"github.com/gambol99/embassy/proxy/endpoints"
	"github.com/golang/glog"
	"github.com/gambol99/embassy/proxy/loadbalancer"
)

const SO_ORIGINAL_DST = 80

/*
Create the proxy service - the main routine for handling requests and events
 */
func NewProxyService(cfg Configuration, store services.ServiceStore) (ProxyService, error) {
	glog.Infof("Initializing the ProxyService [config] => %s", cfg )
	service := new(ProxyStore)
	service.Config = cfg
	service.Store = store
	/* step: we need to grab the ip address of the interface to bind to */
	ipaddress, err := utils.GetLocalIPAddress(cfg.Interface)
	if err != nil {
		glog.Error("Unable to get the local ip address from interface: %s, error: %s", cfg.Interface, err )
		return nil, err
	}
	/* step: setup and initialize the rest */
	service.Proxies = make(map[ProxyID]ServiceProxy, 0)
	service.Shutdown = make(utils.ShutdownSignalChannel)

	glog.Infof("Binding proxy service to interface: %s:%d", ipaddress, cfg.ProxyPort )
	listener , err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.ProxyPort ) )
	if err != nil {
		glog.Errorf("Failed to bind the proxy service to interface, error: %s", err)
		return nil, err
	}
	service.Listener = listener
	return service, nil
}

func NewServiceProxy(si services.Service, discovery string) (ServiceProxy, error) {
	glog.Infof("Initializing a new service proxy for service: %s, discovery: %s", si, discovery )

	proxy := new(Proxier)
	proxy.Service = si
	if balancer, err := loadbalancer.NewLoadBalancer("rr"); err != nil {
		glog.Errorf("Failed to create load balancer for proxier, service: %s, error: %s", si, err)
		return nil, err
	} else {
		proxy.Balancer = balancer
	}
	/* step: create a endpoints store for this service */
	if endpoints, err := endpoints.NewEndpointsService(discovery, si); err != nil {
		glog.Errorf("Failed to create discovery agent on proxier, service: %s, error: %s", si, err)
		return nil, err
	} else {
		proxy.Endpoints = endpoints
		if err = proxy.Endpoints.Synchronize(); err != nil {
			glog.Errorf("Failed to synchronize the endpoints on proxier startup, error: %s", err)
		}
		/* step: start the discovery agent watcher */
		proxy.Endpoints.WatchEndpoints()
	}
	/* step: handle the events */
	proxy.ProcessEvents()
	return proxy, nil
}
