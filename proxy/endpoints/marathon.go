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
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"sync"

	"github.com/gambol99/embassy/config"
	"github.com/gambol99/embassy/proxy/services"
	"github.com/golang/glog"
)

var (
	/* the lock is used to register a callback provider on first service request -
		@todo - we should probably do this on startup rather than waiting
	*/
	marathon_lock sync.Once
	/* the reference to the endpoint provider */
	marathon Marathon
)

/* ===================================================================
	Marathon Endpoints Client
   =================================================================== */

type MarathonClient struct {
	/* a channel to receive updates from the endpoint from */
	update_channel EndpointEventChannel
	/* a shutdown channel */
	shutdown_channel chan bool
}

func NewMarathonClient(uri string) (EndpointsProvider, error) {
	glog.Infof("Creating Marathon discovery agent, marathon: %s", uri)
	/* step: check if endpoint has been create yet and if not create it */
	marathon_lock.Do(func() {
		/* step: we need to register a endpoint for marathon events */
		var err error
		marathon, err = NewMarathonEndpoint()
		if err != nil {
			glog.Fatalf("Failed to register with marathon events service, no point in continuing, error: %s", err)
		}
		glog.Infof("Successfully registered a Marathon Endpoint on: %s", marathon.GetCallbackURL())
	})
	/* step: extract the marathon url */
	service := new(MarathonClient)
	service.update_channel = make(EndpointEventChannel)
	service.shutdown_channel = make(chan bool)
	return service, nil
}

func (r *MarathonClient) List(service *services.Service) ([]Endpoint, error) {
	glog.V(5).Infof("Retrieving a list of endpoints from marathon: %s", config.Options.Discovery_url)
	/* step: extract the service id */
	if name, _, err := r.ServiceID(string(service.ID)); err != nil {
		glog.Errorf("Failed to retrieve the service port, error: %s", err)
		return nil, err
	} else {
		if application, err := marathon.Application(name); err != nil {
			glog.Errorf("Failed to retrieve a list of tasks for application: %s, error: %s", service.ID, err)
			return nil, err
		} else {
			/* step: iterate the tasks and build the endpoints */
			endpoints := make([]Endpoint,0)
			/* check: does the application have any active tasks ? */
			if len(application.Tasks) <= 0 {
				return endpoints, nil
			}
			/* check: is the port we want exposed by the application? */
			if len(application.Ports) <= 0 {
				glog.Errorf("The marathon application: %s does not have any port exposed or mapped", application.ID)
				return endpoints, nil
			}

			/* step: iterate the tasks and extract the ports */
			port_index := -1
			glog.Infof("Application: %s, ports: %v", application.ID, application.Ports)

			/* check: check we've decoded the docker */
			for index, port_mapping := range application.Container.Docker.PortMappings {
				if port_mapping.ContainerPort == service.Port {
					port_index = index
				}
			}
			/* check: did we find the port? */
			if port_index < 0 {
				glog.Errorf("The port: %s is not presenly exposed by the marathon application: %s", service.Port, application.ID)
				return endpoints, errors.New("The service port is not presently exposed by the application: " + application.ID)
			}
			/*
				step: we iterate the tasks and extract the ports - THIS IS SUCH A SHITTY WAY of representing the
				ports mapping in marathon, HONESTLY!! ... Can i not get a bloody hash!!!
			 */
			for _, task := range application.Tasks {
				endpoints = append(endpoints, Endpoint(fmt.Sprintf("%s:%d", task.Host, task.Ports[port_index])))
			}
			glog.V(5).Infof("Found %d endpoints in marathon application: %s, endpoints: %v",
				len(endpoints), application.ID, endpoints)
			return endpoints, nil
		}
	}
}

func (r *MarathonClient) Watch(service *services.Service) (EndpointEventChannel, error) {
	/*
		step: validate the service definition, due to the internal representation of applications in
		marathon, the service port *MUST* is specified in the service definition i.e. BACKEND_FE=/prod/frontend/80;80
	*/
	if name, port, err := r.ServiceID(string(service.ID)); err != nil {
		glog.Errorf("Failed to retrieve the service port, error: %s", err)
		return nil, err
	} else {
		/* step: register for the service */
		glog.V(5).Infof("Registering for marathon events for service: %s:%d", name, port)
		marathon.Watch(name, port, r.update_channel)
		/* step: wait for events from the service */
		go func() {
			for {
				select {
				case event :=<- r.update_channel:
					var _ = event

				case <- r.shutdown_channel:

				}
			}
		}()
	}
	return nil, nil
}

func (r *MarathonClient) Close() {
	/* step: we need to remove our self from listening to the event from the marathon endpoint service */
}

var MarathonServiceRegex = regexp.MustCompile("^(.*)/([0-9]+);([0-9]+)")

func (r *MarathonClient) ServiceID(service_id string) (string, int, error) {
	if elements := MarathonServiceRegex.FindAllStringSubmatch(service_id, -1); len(elements) > 0 {
		section := elements[0]
		service_name := section[1]
		service_port, _ := strconv.Atoi(section[1])
		return service_name, service_port, nil
	} else {
		glog.Errorf("The service definition for service: %s, when using marathon as a provider must have a service port", service_id)
		return "", 0, errors.New("The service definition is invalid, please check documentation regarding marathon provider")
	}
}
