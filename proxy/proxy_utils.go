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

func NewProxyService(cfg *config.Configuration, service services.Service) (ProxyService, error) {
	glog.V(5).Infof("Creating new proxy service for: %s", service)
	proxier := new(Proxier)
	proxier.Service = service
	/* step: create a load balancer for the service */
	balancer, err := NewLoadBalancer("rr")
	if err != nil {
		glog.Errorf("Unable to create a load balancer for service: %s, error: %s", service, err)
		return nil, err
	}
	proxier.LoadBalancer = balancer

	/* step: create a proxy socket for this service */
	socket, err := NewProxySocket(service.Protocol, service.Port)
	if err != nil {
		glog.Errorf("Unable to create a proxy socket for service: %s, error: %s", service, err)
		return nil, err
	}
	proxier.Socket = socket

	/* step: create the discovery agent */
	err = InitializeProxyDiscoveryService(cfg, proxier)
	if err != nil {
		glog.Errorf("Unable to initialize a discovery agent on the service: %s", proxier.Service)
		return nil, err
	}
	return proxier, nil
}

func InitializeProxyDiscoveryService(cfg *config.Configuration, px *Proxier) error {
	glog.Infof("Engaging the disvovery service for proxy")
	/* step: create a service discovery agent on this service */
	px.DiscoveryChannel = make(discovery.DiscoveryStoreChannel, 10)
	discovery, err := discovery.NewDiscoveryService(cfg, px.Service)
	if err != nil {
		glog.Errorf("Unable to create a discovery store for service: %s, error: %s", px.Service, err)
		return err
	}
	px.Discovery = discovery
	/* step: attempt to synchronize the endpoints now */
	err = px.Discovery.Synchronize()
	if err != nil {
		glog.Errorf("Failed to perform initial synchronization of endpoints, error: %s", err)
	}
	/* step: start the watcher routine */
	glog.V(3).Infof("Starting the discovery endpoint watch, service: %s", px.Service)
	px.Discovery.WatchEndpoints(px.DiscoveryChannel)
	return nil
}

func NewProxySocket(protocol services.ServiceProtocol, port int) (ProxySocket, error) {
	switch protocol {
	case services.TCP:
		listener, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(port))
		if err != nil {
			glog.Errorf("Unable to create a TCP proxy socket, error: %s", err)
			return nil, err
		}
		return &TCPProxySocket{listener}, nil
	case services.UDP:
		addr, err := net.ResolveUDPAddr("udp", "127.0.0.1:"+strconv.Itoa(port))
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
		outConn, err := net.DialTimeout(service.ProtocolName(), string(endpoint), retryTimeout*time.Second)
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
	src.Close()
	dest.Close()
	glog.V(4).Infof("Copied %d bytes %s: %s -> %s", n, direction, src.RemoteAddr(), dest.RemoteAddr())
}
