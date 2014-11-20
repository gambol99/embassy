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

	"github.com/gambol99/embassy/services"
	"github.com/golang/glog"
)

type LoadBalancerRR struct {
	LastIndex int
}

func NewLoadBalancerRR() LoadBalancer {
	return &LoadBalancerRR{}
}

func (lb *LoadBalancerRR) SelectEndpoint(service *services.Service, endpoints []services.Endpoint) (services.Endpoint, error) {
	glog.V(3).Infof("Load (RR): selecting endpoint service: %s", service)
	if len(endpoints) <= 0 {
		return "", errors.New("The service does not have any endpoints")
	} else {
		lb.LastIndex++
		if lb.LastIndex > len(endpoints) {
			lb.LastIndex = 0 // reset the index to the begining
		}
		return endpoints[lb.LastIndex], nil
	}
}

func (lb *LoadBalancerRR) UpdateEndpoints(service *services.Service, endpoints []services.Endpoint) {
	glog.V(2).Infof("lb (rr) : updating the endpoints")
	if len(endpoints) > 0 {
		lb.LastIndex = 0
	}
}
