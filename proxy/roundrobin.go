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
	"sync"

	"github.com/gambol99/embassy/services"
	"github.com/golang/glog"
)

type LoadBalancerRR struct {
	sync.Mutex
	NextEndpointIndex int
}

func NewLoadBalancerRR() LoadBalancer {
	return new(LoadBalancerRR)
}

func (lb *LoadBalancerRR) SelectEndpoint(service *services.Service, endpoints []services.Endpoint) (services.Endpoint, error) {
	glog.V(6).Infof("Load (RR): selecting endpoint service: %s, last index: %d, endpoints: %V", service, lb.NextEndpointIndex, endpoints)
	if len(endpoints) <= 0 {
		return "", errors.New("The service does not have any endpoints")
	}
	lb.Lock()
	defer lb.Unlock()
	if lb.NextEndpointIndex > len(endpoints)-1 {
		glog.V(5).Infof("Load (RR) Resetting the index back to begining")
		lb.NextEndpointIndex = 0
	}
	endpoint := endpoints[lb.NextEndpointIndex]
	lb.NextEndpointIndex++
	return endpoint, nil
}

func (lb *LoadBalancerRR) UpdateEndpoints(service *services.Service, endpoints []services.Endpoint) {
	glog.V(2).Infof("lb (rr) : updating the endpoints")
	if len(endpoints) > 0 {
		lb.NextEndpointIndex = 0
	}
}
