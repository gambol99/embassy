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

package proxy

/*
	Service Proxies are shared across multiple endpoints - thus, if you have docker A and docker B
	both talking to the service redis-slave;6379. All their connections are channelled through the
	same service proxy (service_proxy.go). The mapping from Proxies is

	So for example;

		dockerA (172.17.0.27): redis-slave;6379
		dockerB (172.17.0.28): redis-slave;6379
		dockerB (172.17.0.29): redis-slave;6380

	Proxies["172.17.0.27:6379"] -> service_proxyA
	Proxies["172.17.0.28:6379"] -> service_proxyA
	Proxies["172.17.0.28:6380"] -> service_proxyA
 */

import (
	"errors"
	"sync"

	"github.com/gambol99/embassy/proxy/endpoints"
	"github.com/gambol99/embassy/proxy/services"
	"github.com/golang/glog"
)

type ServiceMap interface {
	/* the size of the service map */
	Size() int
	/* lookup the service proxy by service id */
	LookupProxyByServiceId(services.ServiceID) (ServiceProxy, bool)
	/* lookup the service proxy by proxy id */
	LookupProxyByProxyID(ProxyID) (ServiceProxy, bool)
	/* create a new service proxy */
	CreateServiceProxy(si services.Service) error
	/* delete a service proxy */
	DestroyServiceProxy(si services.Service) error
	/* list of the proxier */
	ListProxies() []ServiceProxy
	/* get a list of services */
	ListServices() map[services.ServiceID]services.Service
	/* get a list of endpoints for a service */
	ListServiceEndpoints(string) ([]endpoints.Endpoint, error)
}

type ProxyServiceMap struct {
	/* a service lock */
	sync.RWMutex
	/* a map of the proxy [source_ip+service_port] to the proxy service handler */
	Proxies map[ProxyID]ServiceProxy
}

func NewProxyServices() ServiceMap {
	return &ProxyServiceMap{Proxies: make(map[ProxyID]ServiceProxy, 0)}
}

func (r *ProxyServiceMap) Size() int {
	r.RUnlock()
	defer r.RUnlock()
	return len(r.Proxies)
}

func (r *ProxyServiceMap) CreateServiceProxy(si services.Service) error {
	/* step: wait for a global lock on the services */
	r.Lock()
	defer r.Unlock()

	/* step: get the proxy id for this service */
	proxyID := GetProxyIDByService(&si)

	/* step: check if a service proxy already exists */
	if proxy, found := r.LookupProxyByServiceId(si.ID); found {
		// the proxy service already exists, we simply map a proxyID -> ProxyService
		glog.V(4).Infof("A proxy service already exists for service: %s, mapping new proxy id: %s", si, proxyID)
		r.AddServiceProxy(proxyID, proxy)
	} else {
		/* step: we need to create a new service proxy for this service */
		glog.Infof("Creating new service proxy for service: %s, consumer: %s", si, proxyID)
		proxy, err := NewServiceProxy(si)
		if err != nil {
			glog.Errorf("Unable to create proxier, service: %s, error: %s", si, err)
			return err
		}
		/* step; add the new proxier to the collection */
		r.AddServiceProxy(proxyID, proxy)
	}
	return nil
}

func (r *ProxyServiceMap) DestroyServiceProxy(si services.Service) error {
	r.Lock()
	defer r.Unlock()

	/* step: get the proxy id for this service */
	proxyID := GetProxyIDByService(&si)

	count := 0
	for _, proxy := range r.Proxies {
		if proxy.GetService().ID == si.ID {
			count++
		}
	}
	/* step: does we have multiple mappings */
	if count == 0 {
		glog.Errorf("Unable to find a service proxy for service: %s", si)
		return errors.New("Service Proxy does not exists")
	}
	if count == 1 {
		glog.Infof("Releasing the resources of service proxy, service: %s", si)
		proxy, _ := r.Proxies[proxyID]
		proxy.Close()
	} else {
		glog.Infof("Service proxy for service: %s has multiple mappings", si)
	}
	delete(r.Proxies, proxyID)
	return nil
}

func (r *ProxyServiceMap) ListProxies() []ServiceProxy {
	r.RLock()
	defer r.RUnlock()
	list := make([]ServiceProxy, 0)
	for _, proxy := range r.Proxies {
		list = append(list, proxy)
	}
	return list
}

func (r *ProxyServiceMap) ListServices() map[services.ServiceID]services.Service {
	r.RLock()
	defer r.RUnlock()
	list := make(map[services.ServiceID]services.Service, 0)
	for _, proxy := range r.Proxies {
		if service, found := list[proxy.GetService().ID]; !found {
			list[proxy.GetService().ID] = service
		}
	}
	return list
}

func (r *ProxyServiceMap) ListServiceEndpoints(string) ([]endpoints.Endpoint, error) {
	r.RLock()
	defer r.RUnlock()
	list := make([]endpoints.Endpoint, 0)
	return list, nil
}

func (r *ProxyServiceMap) LookupProxyByServiceId(id services.ServiceID) (ServiceProxy, bool) {
	for _, service := range r.Proxies {
		if service.GetService().ID == id {
			glog.V(4).Infof("Found service proxy for service id: %s", id)
			return service, true
		}
	}
	glog.V(4).Infof("Proxy service does not exists for service id: %s", id)
	return nil, false
}

func (r *ProxyServiceMap) LookupProxyByProxyID(id ProxyID) (proxy ServiceProxy, found bool) {
	proxy, found = r.Proxies[id]
	return
}

func (r *ProxyServiceMap) AddServiceProxy(proxyID ProxyID, proxy ServiceProxy) {
	glog.V(8).Infof("Adding Service Proxy, service: %s, service id: %s", proxy.GetService(), proxy.GetService().ID)
	r.Proxies[proxyID] = proxy
	glog.V(3).Infof("Added proxyId: %s to collection of service proxies, size: %d", proxyID, len(r.Proxies))
}
