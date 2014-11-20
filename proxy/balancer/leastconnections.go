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
	"github.com/gambol99/embassy"
	"github.com/gambol99/embassy/proxy/loadbalancer"
	"github.com/golang/glog"
)

type ConnectionCount int

func (count *ConnectionCount) Increment() { *count++ }
func (count *ConnectionCount) Decrement() {
	*count--
	if *count <= 0 {
		*count = 0
	}
}

type LoadBalancerLC struct {
	Connections map[ServiceEndpointID]ConnectionCount
}

func NewLeastConnections() LoadBalancer {
	balancer := new(LoadBalancerLC)
	balancer.Connections = make(map[ServiceEndpointID]ConnectionCount, 0)
	return balancer
}

func (r *LoadBalancerLC) SelectEndpoint(service *Service, endpoints []ServiceEndpoint) (*ServiceEndpoint, error) {
	glog.V(3).Infof("Load (LC): selecting endpoint service: %s", service)

	return nil, nil
}

func (lb *LoadBalancerLC) UpdateEndpoints(service *Service, endpoints []ServiceEndpoint) {

}
