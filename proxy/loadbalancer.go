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
)

type LoadBalancer interface {
	/* select a endpoint for this service */
	SelectEndpoint(service *services.Service, endpoints []services.Endpoint) (*services.Endpoint, error)
	/* update the endpoints */
	UpdateEndpoints(service *services.Service, endpoints []services.Endpoint)
}

const (
	DEFAULT_BALANCER_NAME = "rr"
)

func NewLoadBalancer(name string) (LoadBalancer, error) {
	if name == "" {
		name = DEFAULT_BALANCER_NAME
	}
	lb := map[string]LoadBalancer{
		"rr": NewLoadBalancerRR(),
		"lc": NewLeastConnections(),
	}[name]
	if lb == nil {
		return nil, errors.New("Unable to find the specified load balancer")
	}
	return lb, nil
}
