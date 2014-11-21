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
	"net"
	"sync"
	"time"

	"github.com/gambol99/embassy/discovery"
	"github.com/gambol99/embassy/services"
	"github.com/golang/glog"
)

type TCPProxySocket struct {
	net.Listener
}

func (tcp *TCPProxySocket) ProxyService(service *services.Service, balancer LoadBalancer, discovery discovery.DiscoveryStore) error {
	glog.V(5).Infof("TCP Proxy Service, service: %s", service)
	for {
		/* wait for a connection */
		connection, err := tcp.Accept()
		if err != nil {
			glog.Errorf("Accept connection failed: %s", err)
			continue
		}
		glog.V(2).Infof("Accepted TCP connection from %v to %v", connection.RemoteAddr(), connection.LocalAddr())
		/* step: attempt to connect to a backend in a goroutine */
		go tcp.HandleTCPConnection(service, connection, balancer, discovery)
	}
}

func (p *TCPProxySocket) HandleTCPConnection(service *services.Service, inConn net.Conn, balancer LoadBalancer, discovery discovery.DiscoveryStore) error {
	/* step: we try and connect to a backend */
	outConn, err := TryConnect(service, balancer, discovery)
	defer func() {
		glog.V(5).Infof("Closing the connection from: %v", inConn.RemoteAddr())
		inConn.Close()
	}()
	/* step: set some deadlines */
	inConn.SetReadDeadline(time.Now().Add(5 * time.Second))
	if err != nil {
		glog.Errorf("Failed to connect to balancer: %v", err)
		return err
	}
	defer outConn.Close()
	/* step: we spin up to async routines to handle the byte transfer */
	var wg sync.WaitGroup
	wg.Add(2)
	go TransferTCPBytes("->", inConn, outConn, &wg)
	go TransferTCPBytes("<-", outConn, inConn, &wg)
	wg.Wait()
	return nil
}
