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
	"github.com/gambol99/embassy/services"
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
	Connections map[services.Endpoint]ConnectionCount
}

func NewLeastConnections() LoadBalancer {
	balancer := new(LoadBalancerLC)
	balancer.Connections = make(map[services.Endpoint]ConnectionCount, 0)
	return balancer
}

func (r *LoadBalancerLC) SelectEndpoint(service *services.Service, endpoints []services.Endpoint) (services.Endpoint, error) {
	glog.V(3).Infof("Load (LC): selecting endpoint service: %s", service)

	return "", nil
}

func (lb *LoadBalancerLC) UpdateEndpoints(service *services.Service, endpoints []services.Endpoint) {

}
