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

	"github.com/gambol99/embassy/config"
	"github.com/gambol99/embassy/services"
	"github.com/golang/glog"
	"github.com/gambol99/embassy/endpoints"
)

const SO_ORIGINAL_DST = 80

func NewProxier(cfg *config.Configuration, si services.Service) (ServiceProxy, error) {
	proxier := new(Proxier)
	proxier.ID = GetProxyIDByService(&si)
	glog.Infof("Creating a new proxier, service: %s, proxyID: %s", si, proxier.ID)
	/* step: create a load balancer on the service */
	balancer, err := NewLoadBalancer("rr")
	if err != nil {
		glog.Errorf("Failed to create load balancer for proxier, service: %s, error: %s", si, err)
		return nil, err
	}
	proxier.Balancer = balancer
	/* step: create a discovery agent on the proxier service */
	discovery, err := endpoints.NewDiscoveryService(cfg, si)
	if err != nil {
		glog.Errorf("Failed to create discovery agent on proxier, service: %s, error: %s", si, err)
		return nil, err
	}
	/* step: synchronize the endpoints */
	proxier.Discovery = discovery
	if err = proxier.Discovery.Synchronize(); err != nil {
		glog.Errorf("Failed to synchronize the endpoints on proxier startup, error: %s", err)
	}
	/* step: start the discovery agent watcher */
	proxier.Discovery.WatchEndpoints()
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
