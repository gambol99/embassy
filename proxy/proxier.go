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
	"io"
	"net"
	"sync"
	"time"

	"github.com/gambol99/embassy/endpoints"
	"github.com/gambol99/embassy/services"
	"github.com/golang/glog"
)

var endpointDialTimeout = []time.Duration{1, 2, 4}

type Proxier struct {
	/* the service the proxy is proxying for */
	Service   services.Service
	/* the discovery agent for this service */
	EndpointStore endpoints.EndpointsStore
	/* the load balancer for this service */
	Balancer  LoadBalancer
	/* the shutdown channel for the service */
}

type ServiceProxy interface {
	/* close all the assets associated to this service */
	Close();
	/* handle a inbound connection */
	HandleTCPConnection(*net.TCPConn) error
}

func (r *Proxier) ID() {
	return r.ID
}

func (r *Proxier) HandleTCPConnection(inConn *net.TCPConn) error {
	/* step: we try and connect to a backend */
	outConn, err := r.TryConnect()
	/* step: set some deadlines */
	if err != nil {
		glog.Errorf("Failed to connect to balancer: %v", err)
		inConn.Close()
		return err
	}
	/* step: we spin up to async routines to handle the byte transfer */
	var wg sync.WaitGroup
	wg.Add(2)
	go TransferTCPBytes("->", inConn, outConn, &wg)
	go TransferTCPBytes("<-", outConn, inConn, &wg)
	wg.Wait()
	inConn.Close()
	outConn.Close()
	return nil
}

func (r *Proxier) TryConnect() (backend *net.TCPConn, err error) {
	/* step: attempt multiple times to connect to backend */
	for _, retryTimeout := range endpointDialTimeout {
		endpoints, err := r.EndpointStore.ListEndpoints()
		if err != nil {
			glog.Errorf("Unable to retrieve any endpoints for service: %s, error: %s", r.Service, err)
			return nil, err
		}
		/* step: we get a service endpoint from the load balancer */
		endpoint, err := r.Balancer.SelectEndpoint(endpoints)
		if err != nil {
			glog.Errorf("Unable to find an service endpoint for service: %s", r.Service, err)
			return nil, err
		}
		glog.V(4).Infof("Proxying service %s to endpoint %s", r.Service, endpoint.Service)
		/* step: attempt to connect to the backend */
		outConn, err := net.DialTimeout(r.Service.Protocol(), string(endpoint.Service), retryTimeout*time.Second)
		if err != nil {
			glog.Errorf("Failed to connect to backend service: %s, error: %s", endpoint, err)
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
	n, err := io.Copy(dest, src)
	if err != nil {
		glog.Errorf("I/O error: %v", err)
	}
	glog.V(4).Infof("Copied %d bytes %s: %s -> %s", n, direction, src.RemoteAddr(), dest.RemoteAddr())
	dest.CloseWrite()
	src.CloseRead()
}
