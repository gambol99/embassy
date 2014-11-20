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

package proxy

import (
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/gambol99/embassy/config"
	"github.com/gambol99/embassy/discovery"
	"github.com/gambol99/embassy/services"
	"github.com/golang/glog"
)

var endpointDialTimeout = []time.Duration{1, 2, 4, 8}

func NewProxyService(config *config.ServiceConfiguration, service services.Service) (ProxyService, error) {
	proxy := new(Proxier)
	/* step: create discovery channel for this service */
	proxy.DiscoveryChannel = make(discovery.DiscoveryStoreChannel, 10)

	/* step: create a load balancer for the service */
	balancer, err := NewLoadBalancer("rr")
	if err != nil {
		glog.Errorf("Unable to create a load balancer for service: %s, error: %s", service, err)
		return nil, err
	}
	proxy.LoadBalancer = balancer

	/* step: create a service discovery agent on this service */
	discovery, err := discovery.NewDiscoveryService(config, service)
	if err != nil {
		glog.Errorf("Unable to create a discovery store for service: %s, error: %s", service, err)
		return nil, err
	}
	proxy.Discovery = discovery

	/* step: create a proxy socket for this service */
	socket, err := NewProxySocket(service.Protocol, service.Port)
	if err != nil {
		glog.Errorf("Unable to create a proxy socket for service: %s, error: %s", service, err)
		return nil, err
	}
	proxy.Socket = socket
	return proxy, nil
}

func NewProxySocket(protocol services.ServiceProtocol, port int) (ProxySocket, error) {
	switch protocol {
	case services.TCP:
		listener, err := net.Listen("tcp", ":"+strconv.Itoa(port))
		if err != nil {
			glog.Errorf("Unable to create a TCP proxy socket, error: %s", err)
			return nil, err
		}
		return &TCPProxySocket{listener}, nil
	case services.UDP:
		addr, err := net.ResolveUDPAddr("udp", ":"+strconv.Itoa(port))
		if err != nil {
			return nil, err
		}
		conn, err := net.ListenUDP("udp", addr)
		if err != nil {
			return nil, err
		}
		return &UDPProxySocket{conn}, nil
	}
	return nil, fmt.Errorf("Unknown protocol %q", protocol)
}

func TryConnect(service *services.Service, lb LoadBalancer, ds discovery.DiscoveryStore) (backend net.Conn, err error) {
	/* step: attempt multiple times to connect to backend */
	for _, retryTimeout := range endpointDialTimeout {
		endpoints, err := ds.ListEndpoints()
		if err != nil {
			glog.Errorf("Unable to retrieve any endpoints for service: %s, error: %s", service, err)
			return nil, err
		}
		/* step: we get a service endpoint from the load balancer */
		endpoint, err := lb.SelectEndpoint(service, endpoints)
		if err != nil {
			glog.Errorf("Unable to find an service endpoint for service: %s", service, err)
			return nil, err
		}
		glog.V(3).Infof("Proxying service %s to endpoint %s", service, endpoint)
		/* step: attempt to connect to the backend */
		outConn, err := net.DialTimeout(service.ProtocolName(), endpoint.Address.String(), retryTimeout*time.Second)
		if err != nil {
			glog.Errorf("Dial failed: %v", err)
			continue
		}
		return outConn, nil
	}
	glog.Errorf("Unable to connect service: %s to any endpoints", service)
	return nil, errors.New("Unable to connect to any endpoints")
}

func TransferTCPBytes(direction string, dest, src net.Conn, waitgroup *sync.WaitGroup) {
	defer waitgroup.Done()
	glog.V(4).Infof("Copying %s: %s -> %s", direction, src.RemoteAddr(), dest.RemoteAddr())
	n, err := io.Copy(dest, src)
	if err != nil {
		glog.Errorf("I/O error: %v", err)
	}
	glog.V(4).Infof("Copied %d bytes %s: %s -> %s", n, direction, src.RemoteAddr(), dest.RemoteAddr())
}
