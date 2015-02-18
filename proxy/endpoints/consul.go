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

package endpoints

import (
	"fmt"
	"net/url"
	"time"

	consulapi "github.com/armon/consul-api"
	"github.com/gambol99/embassy/proxy/services"
	"github.com/golang/glog"
	"github.com/gambol99/embassy/config"
)

type ConsulClient struct {
	/* the consul api client */
	client *consulapi.Client
	/* the current wait index */
	wait_index uint64
	/* the kill off */
	kill_off bool
}

const (
	DEFAULT_WAIT_TIME = 10 * time.Second
)

func NewConsulClient(discovery string) (EndpointsProvider, error) {
	config := consulapi.DefaultConfig()
	uri, err := url.Parse(discovery)
	if err != nil {
		glog.Errorf("Failed to parse the discovery url, error: %s", err)
		return nil, err
	}
	if uri.Host != "" {
		config.Address = uri.Host
	}
	client, err := consulapi.NewClient(config)
	return &ConsulClient{client, uint64(0), false}, nil
}

/*
	watch for changes in the consul backend service - note, this probably isn't the best way of
	doing it, though i've not spent much time looking at the api
*/
func (r *ConsulClient) Watch(si *services.Service) (EndpointEventChannel, error) {
	/* channel to send back events to the endpoints store */
	endpointUpdateChannel := make(EndpointEventChannel, 5)
	go func() {
		/* step: we get the catalog */
		catalog := r.client.Catalog()
		for {
			if r.kill_off {
				glog.V(3).Infof("Terminating the consul watcher on service: %s", si.Name)
				break
			}
			if r.wait_index == 0 {
				/* step: get the wait index for the service */
				_, meta, err := catalog.Service(si.Name, "", &consulapi.QueryOptions{})
				if err != nil {
					glog.Errorf("Failed to grab the service fron consul, error: %s", err)
					time.Sleep(5 * time.Second)
				} else {
					/* update the wait index for this service */
					r.wait_index = meta.LastIndex
					glog.V(8).Infof("Last consul index for service: %s was index: %d", si.Name, meta.LastIndex)
				}
			}
			/* step: build the query - make sure we have a timeout */
			queryOptions := &consulapi.QueryOptions{
				WaitIndex: r.wait_index,
				WaitTime:  DEFAULT_WAIT_TIME}

			/* step: making a blocking watch call for changes on the service */
			_, meta, err := catalog.Service(si.Name, "", queryOptions)
			if err != nil {
				glog.Errorf("Failed to wait for service to change, error: %s", err)
				r.wait_index = 0
				time.Sleep(5 * time.Second)
			} else {
				if r.kill_off {
					continue
				}
				/* step: if the wait and last index are the same, we can continue */
				if r.wait_index == meta.LastIndex {
					continue
				}
				/* step: update the index */
				r.wait_index = meta.LastIndex

				/* step: construct the change event and send */
				var event EndpointEvent
				event.ID = si.Name
				event.Action = ENDPOINT_CHANGED
				endpointUpdateChannel <- event
			}
		}
		close(endpointUpdateChannel)
	}()
	return endpointUpdateChannel, nil
}

func (r *ConsulClient) List(si *services.Service) ([]Endpoint, error) {
	glog.V(5).Infof("Retrieving a list of the endpoints for service: %s", si)
	health_checks_required := config.Options.Filter_On_Health
	/* step: query for the service, along with health checks */
	if services, _, err := r.client.Health().Service(si.Name, "", health_checks_required, &consulapi.QueryOptions{}); err != nil {
		glog.Errorf("Failed to retrieve a list of services for service: %s", si)
		return nil, err
	} else {
		list := make([]Endpoint, 0)
		/* step: iterate the CatalogService and pull the endpoints */
		for _, service := range services {
			service_address := service.Node.Address
			service_port := service.Service.Port
			endpoint := Endpoint(fmt.Sprintf("%s:%d", service_address, service_port))
			list = append(list, endpoint)
		}
		glog.V(8).Infof("Retrieved the list of endpoints for service: %s, endpoint: %s", si.Name, list)
		return list, nil
	}
}

func (r *ConsulClient) Close() {
	glog.Infof("Request to shutdown the consul agent")
	r.kill_off = true
}
