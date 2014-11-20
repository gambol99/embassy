package main

/*
#
#   Author: Rohith (gambol99@gmail.com)
#   Date: 2014-11-14 18:27:38 +0000 (Fri, 14 Nov 2014)
#
#  vim:ts=2:sw=2:et
#
*/

import (
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
