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
	"syscall"
	"strconv"
	"net"
	"fmt"

	"github.com/gambol99/embassy/utils"
	"github.com/gambol99/embassy/config"
	"github.com/gambol99/embassy/services"
	"github.com/golang/glog"
	"github.com/gambol99/embassy/endpoints"
)

const SO_ORIGINAL_DST = 80

func NewProxyStore(cfg *config.Configuration, store services.ServiceStore) (ProxyService, error) {
	glog.Infof("Creating a new ProxyService")
	proxy := new(ProxyStore)
	proxy.Config = cfg

	/* step: create a channel to listen for new services from the store */
	glog.V(4).Infof("Creating a services channel for the proxy")
	proxy.ServicesChannel = make(services.ServiceStoreChannel)
	store.AddServiceListener(proxy.ServicesChannel)

	/* step: create a tcp listener for the proxy service */
	glog.V(2).Infof("Binding proxy to interface: %s:%d", cfg.IPAddress, cfg.ProxyPort)
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.ProxyPort))
	if err != nil {
		glog.Errorf("Unable to bind proxy service, error: %s", err)
		return nil, err
	}
	proxy.Listener = listener

	/* step: create the map for holder proxiers */
	proxy.Proxies = make(map[ProxyID]ServiceProxy, 0)
	/* step: create the shutdown channel */
	proxy.Shutdown = make(utils.ShutdownSignalChannel)

	/* step: start finding services */
	if err := store.FindServices(); err != nil {
		glog.Errorf("Failed to start the services stream, error: %s", err)
	}
	return proxy, nil
}

func NewServiceProxy(cfg *config.Configuration, si services.Service) (ServiceProxy, error) {
	glog.Infof("Creating a new proxier, service: %s", si )

	proxier := new(Proxier)
	proxier.Service = si
	/* step: create a load balancer on the service */
	balancer, err := NewLoadBalancer("rr")
	if err != nil {
		glog.Errorf("Failed to create load balancer for proxier, service: %s, error: %s", si, err)
		return nil, err
	}
	proxier.Balancer = balancer
	/* step: create a discovery agent on the proxier service */
	endpoints, err := endpoints.NewEndpointsService(cfg, si)
	if err != nil {
		glog.Errorf("Failed to create discovery agent on proxier, service: %s, error: %s", si, err)
		return nil, err
	}
	/* step: synchronize the endpoints */
	proxier.Endpoints = endpoints
	if err = proxier.Endpoints.Synchronize(); err != nil {
		glog.Errorf("Failed to synchronize the endpoints on proxier startup, error: %s", err)
	}
	/* step: start the discovery agent watcher */
	proxier.Endpoints.WatchEndpoints()

	/* step: handle the events */
	proxier.HandleEvents()

	return proxier, nil
}

func GetOriginalPort(conn *net.TCPConn) (string, error) {
	descriptor, err := conn.File()
	if err != nil {
		glog.Errorf("Unable to get tcp descriptor, connection: %s, error: ", conn.RemoteAddr(), err)
		return "", err
	}
	addr, err := syscall.GetsockoptIPv6Mreq(int(descriptor.Fd()), syscall.IPPROTO_IP, SO_ORIGINAL_DST)
	if err != nil {
		glog.Errorf("Unable to get the original destination port for connection: %s, error: %s", conn.RemoteAddr(), err)
		return "", err
	}
	destination := uint16(addr.Multiaddr[2])<<8 + uint16(addr.Multiaddr[3])
	return strconv.Itoa(int(destination)), nil
}
