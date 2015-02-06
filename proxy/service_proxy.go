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
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/gambol99/embassy/config"
	"github.com/gambol99/embassy/proxy/endpoints"
	"github.com/gambol99/embassy/proxy/loadbalancer"
	"github.com/gambol99/embassy/proxy/services"
	"github.com/gambol99/embassy/utils"
	"github.com/golang/glog"
)

var endpointDialTimeout = []time.Duration{2, 4, 8}

type ServiceProxy interface {
	/* close all the assets associated to this service */
	Close()
	/* handle a inbound connection */
	HandleTCPConnection(*net.TCPConn) error
	/* retrieve the service associated */
	GetService() services.Service
}

type Proxier struct {
	/* the service the proxy is proxying for */
	Service services.Service
	/* the discovery agent for this service */
	Endpoints endpoints.EndpointsStore
	/* the load balancer for this service */
	Balancer loadbalancer.LoadBalancer
	/* the shutdown signal */
	Shutdown utils.ShutdownSignalChannel
}

func NewServiceProxy(service services.Service) (ServiceProxy, error) {
	glog.Infof("Initializing a new service proxy for service: %s, discovery: %s", service, config.Options.Discovery_url)

	/* step: creating the service proxy */
	proxy := new(Proxier)
	proxy.Service = service
	proxy.Shutdown = make(utils.ShutdownSignalChannel)
	if balancer, err := loadbalancer.NewLoadBalancer("rr"); err != nil {
		glog.Errorf("Failed to create load balancer for proxier, service: %s, error: %s", service, err)
		return nil, err
	} else {
		proxy.Balancer = balancer
	}

	/* step: create a endpoints store for this service */
	if endpoints, err := endpoints.NewEndpointsService(config.Options.Discovery_url, service); err != nil {
		glog.Errorf("Failed to create discovery agent on proxier, service: %s, error: %s", service, err)
		return nil, err
	} else {
		proxy.Endpoints = endpoints
		/* step: we can synchronize the endpoints in the backend */
		go proxy.Endpoints.Synchronize()
		/* step: start the discovery agent watcher */
		proxy.Endpoints.WatchEndpoints()
	}

	/* step: handle the events */
	proxy.ProcessEvents()
	return proxy, nil
}

func (px Proxier) String() string {
	return fmt.Sprintf("service: %s", px.Service)
}

func (r *Proxier) Close() {
	glog.Infof("Destroying the service proxy: %s", r)
	r.Shutdown <- true
}

func (r *Proxier) GetService() services.Service {
	return r.Service
}

func (r *Proxier) ProcessEvents() {
	glog.V(4).Infof("Starting to handle event for service proxy: %s", r)
	/* step: add a event listener to endpoints */
	endpointsChannel := make(endpoints.EndpointEventChannel, 0)
	r.Endpoints.AddEventListener(endpointsChannel)
	go func() {
		defer close(endpointsChannel)
		for {
			select {
			case <-r.Shutdown:
				glog.Infof("Shutting the Service Proxy for service: %s", r.Service)
				r.Endpoints.Close()
				return
			case <-endpointsChannel:
				glog.V(4).Infof("Endpoints for service: %s updated, synchronizing endpoints", r.Service)
				if endpoints, err := r.Endpoints.ListEndpoints(); err != nil {
					glog.Errorf("Unable to push endpoint changes upstream to loadbalancer, error: %s", err)
				} else {
					r.Balancer.UpdateEndpoints(endpoints)
				}
			}
		}
	}()
}

func (r *Proxier) HandleTCPConnection(inConn *net.TCPConn) error {
	/* step: we try and connect to a backend */
	if outConn, err := r.TryConnect(); err != nil {
		glog.Errorf("Failed to connect to balancer: %v", err)
		inConn.Close()
		return err
	} else {
		/* step: we spin up to async routines to handle the byte transfer */
		var wg sync.WaitGroup
		wg.Add(2)
		go TransferTCPBytes("->", inConn, outConn, &wg)
		go TransferTCPBytes("<-", outConn, inConn, &wg)
		wg.Wait()
		inConn.Close()
		outConn.Close()
	}
	return nil
}

func (r *Proxier) TryConnect() (backend *net.TCPConn, err error) {
	/* step: attempt to connect to endpoint */
	for _, retryTimeout := range endpointDialTimeout {
		/* step: we get the endpoints for this service */
		endpoints, err := r.Endpoints.ListEndpoints()
		if err != nil {
			glog.Errorf("Unable to retrieve any endpoints for service: %s, error: %s", r.Service, err)
			return nil, err
		}

		/* step: we select an endpoint from the list */
		endpoint, err := r.Balancer.SelectEndpoint(endpoints)
		if err != nil {
			glog.Errorf("Unable to find an service endpoint for service: %s", r.Service, err)
			return nil, err
		}

		/* step: we use x timeouts to connect to backend */
		glog.V(4).Infof("Proxying service %s to endpoint %s", r.Service, endpoint)

		/* step: attempt to connect to the backend */
		outConn, err := net.DialTimeout("tcp", string(endpoint), retryTimeout*time.Second)
		if err != nil {
			glog.Errorf("Failed to connect to backend service: %s, timeout: %d seconds, error: %s", endpoint, retryTimeout, err)
			continue
		}
		return outConn.(*net.TCPConn), nil
	}
	glog.Errorf("Unable to connect service: %s to any endpoints", r.Service)
	return nil, errors.New("Unable to connect to any endpoints")
}

func TransferTCPBytes(direction string, dest, src *net.TCPConn, waitgroup *sync.WaitGroup) {
	defer waitgroup.Done()
	glog.V(4).Infof("Copying %s: %s -> %s", direction, src.RemoteAddr(), dest.RemoteAddr())
	if n, err := io.Copy(dest, src); err != nil {
		glog.Errorf("I/O error: %v", err)
	} else {
		glog.V(4).Infof("Copied %d bytes %s: %s -> %s", n, direction, src.RemoteAddr(), dest.RemoteAddr())
	}
	dest.CloseWrite()
	src.CloseRead()
}
