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

package loadbalancer

import (
	"errors"
	"sync"

	"github.com/gambol99/embassy/proxy/endpoints"
	"github.com/golang/glog"
)

type LoadBalancerRR struct {
	sync.RWMutex
	NextEndpointIndex int
}

func NewLoadBalancerRR() LoadBalancer {
	return new(LoadBalancerRR)
}

func (lb *LoadBalancerRR) SelectEndpoint(endpoints []endpoints.Endpoint) (endpoints.Endpoint, error) {
	lb.Lock()
	defer lb.Unlock()
	if len(endpoints) <= 0 {
		return "", errors.New("The service does not have any endpoints")
	}
	if lb.NextEndpointIndex > len(endpoints)-1 {
		lb.NextEndpointIndex = 0
	}
	endpoint := endpoints[lb.NextEndpointIndex]
	lb.NextEndpointIndex++
	return endpoint, nil
}

func (lb *LoadBalancerRR) UpdateEndpoints(endpoints []endpoints.Endpoint) {
	glog.V(2).Infof("lb (rr) : updating the endpoints")
	if len(endpoints) > 0 {
		lb.Lock()
		defer lb.Unlock()
		lb.NextEndpointIndex = 0
	}
}
