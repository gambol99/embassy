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
	"net/url"
	"fmt"
	"time"

	consulapi "github.com/armon/consul-api"
	"github.com/gambol99/embassy/utils"
	"github.com/gambol99/embassy/proxy/services"
	"github.com/golang/glog"
)

type ConsulClient struct {
	Client    	*consulapi.Client
	Shutdown 	utils.ShutdownSignalChannel
	WaitIndex 	uint64
	KillOff     bool
}

func NewConsulClent(discovery string) (EndpointsProvider, error) {
	config := consulapi.DefaultConfig()
	uri, err := url.Parse(discovery)
	if err != nil {
		glog.Errorf("Failed to parse the discovery url, error: %s", err )
		return nil, err
	}
	if uri.Host != "" {
		config.Address = uri.Host
	}
	client, err := consulapi.NewClient(config)
	return &ConsulClient{client,make(utils.ShutdownSignalChannel),uint64(0),false}, nil
}

/*
	watch for changes in the consul backend service - note, this probably isn't the best way of
	doing it, though i've not spent much time looking at the api
*/
func (r *ConsulClient) Watch(si *services.Service) (EndpointEventChannel,error) {
	/* channel to send back events to the endpoints store */
	endpointUpdateChannel := make(EndpointEventChannel,5)
	go func() {
		/* step: we get the catalog */
		catalog := r.Client.Catalog()
		for {
			if r.KillOff {
				glog.V(3).Infof("Terminating the consul watcher on service: %s", si.Name )
				return
			}
			if r.WaitIndex == 0 {
				_, meta, err := catalog.Service(si.Name,"",&consulapi.QueryOptions{})
				if err != nil {
					glog.Errorf("Failed to grab the service fron consul, error: %s", err )
					time.Sleep( 5 * time.Second )
				} else {
					r.WaitIndex = meta.LastIndex
					glog.V(8).Infof("Last consul index for service: %s was index: %d", si.Name, meta.LastIndex )
				}
			}
			queryOptions := &consulapi.QueryOptions{WaitIndex: r.WaitIndex }
			_, meta, err := catalog.Service(si.Name,"", queryOptions)
			if err != nil {
				glog.Errorf("Failed to wait for service to change, error: %s", err)
				r.WaitIndex = 0
				time.Sleep( 5 * time.Second )
			} else {
				if r.KillOff {
					continue
				}
				r.WaitIndex = meta.LastIndex
				glog.V(8).Infof("Last wait index for consul, service: %s, index: %s", si, r.WaitIndex )
				var event EndpointEvent
				event.ID = si.Name
				event.Action = ENDPOINT_CHANGED
				endpointUpdateChannel <- event
			}
		}
	}()
	return endpointUpdateChannel, nil
}

func (r *ConsulClient) List(si *services.Service) ([]Endpoint, error) {
	glog.V(5).Infof("Retrieving a list of the endpoints for service: %s", si )
	catalog := r.Client.Catalog()
	services, _, err := catalog.Service(si.Name,"", &consulapi.QueryOptions{WaitIndex: r.WaitIndex})
	if err != nil {
		glog.Errorf("Failed to retrieve a list of services for service: %s", si )
		return nil, err
	}
	list := make([]Endpoint, 0)
	/* step: iterate the CatalogService and pull the endpoints */
	for _, service := range services {
		endpoint := Endpoint(fmt.Sprintf("%s:%d", service.Address, service.ServicePort))
		list = append(list, endpoint)
	}
	return list, nil
}

func (r *ConsulClient) Close() {
	glog.Infof("Request to shutdown the consul agent")
	r.Shutdown <- true
	r.KillOff = true
}


