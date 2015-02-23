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

	"github.com/gambol99/embassy/proxy/endpoints"
	"github.com/golang/glog"
)

type LoadBalancer interface {
	// select a endpoint for this service
	SelectEndpoint(endpoints []endpoints.Endpoint) (endpoints.Endpoint, error)
	// update the endpoints
	UpdateEndpoints(endpoints []endpoints.Endpoint)
}

func NewLoadBalancer(name string) (LoadBalancer, error) {
	glog.V(5).Infof("Creating new load balancer: %s", name)
	lb := map[string]LoadBalancer{
		"rr": NewLoadBalancerRR(),
	}[name]
	if lb == nil {
		return nil, errors.New("Unable to find the specified load balancer")
	}
	return lb, nil
}
