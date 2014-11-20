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

package balancer

import (
	"sync"

	"github.com/gambol99/embassy"
	"github.com/gambol99/embassy/proxy/loadbalancer"
	"github.com/golang/glog"
)

type LoadBalancerRR struct {
	sync.RWMutex
	LastEndpoint int
}

func NewLoadBalancerRR() LoadBalancer {
	return &LoadBalancerRR{}
}

func (lb *LoadBalancerRR) SelectEndpoint(service *Service, endpoints []ServiceEndpoint) (*ServiceEndpoint, error) {
	glog.V(3).Infof("Load (RR): selecting endpoint service: %s", service)
	return nil, nil
}

func (lb *LoadBalancerRR) UpdateEndpoints(service *Service, endpoints []ServiceEndpoint) {
	lb.Lock()
	defer lb.Unlock()
	glog.V(2).Infof("lb (rr) : updating the endpoints")

}
