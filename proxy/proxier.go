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

	"github.com/gambol99/embassy/discovery"
	"github.com/gambol99/embassy/services"
	"github.com/golang/glog"
)

type Proxier struct {
	Service      services.Service
	Discovery    discovery.DiscoveryStore
	LoadBalancer LoadBalancer
	Socket       ProxySocket
	/* --- channels ---- */
	DiscoveryChannel discovery.DiscoveryStoreChannel
}

type ProxyService interface {
	GetService() services.Service
	GetEndpoints() ([]services.Endpoint, error)
	StartServiceProxy()
}

type ProxySocket interface {
	Addr() net.Addr
	Close() error
	ProxyService(*services.Service, LoadBalancer, discovery.DiscoveryStore) error
}

func (p Proxier) GetService() services.Service {
	return p.Service
}

func (p *Proxier) GetEndpoints() ([]services.Endpoint, error) {
	list, err := p.Discovery.ListEndpoints()
	if err != nil {
		glog.Errorf("Unable to retrieve a list of endpoints for service; %s, error: %s", p.Service, err)
		return nil, err
	}
	return list, nil
}

func (p *Proxier) StartServiceProxy() {
	/* step: starting listening on the service port */
	go func(px *Proxier) {
		glog.Infof("Starting proxy service: %s", p.Service)
		px.Socket.ProxyService(&px.Service, px.LoadBalancer, px.Discovery)
	}(p)
	/* step: listen out for events from the channel */
	go func(p *Proxier) {
		for {
			update := <-p.DiscoveryChannel
			glog.V(2).Infof("Recieved discovery update event: %s, reloading the endpoints", update)
			/* step: get a list of endpoints */
			endpoints, err := p.Discovery.ListEndpoints()
			if err != nil {
				glog.Errorf("Unable to pull the latest endpoints from discovery store, error: %s", err)
				continue
			}
			p.LoadBalancer.UpdateEndpoints(&p.Service, endpoints)
			glog.V(3).Infof("Updated the load balancer with the latest endpoints")
		}
	}(p)
}
