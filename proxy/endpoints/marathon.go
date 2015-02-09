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
	/*
		the lock is used to register a callback provider on first service request -
		@todo - we should probably do this on startup rather than waiting
	*/
	marathon_lock sync.Once
	/* the reference to the endpoint provider */
	marathon Marathon
)

var (
	MarathonServiceRegex = regexp.MustCompile("^(.*)/([0-9]+);([0-9]+)")
)

/* ===================================================================
	Marathon Endpoints Client
   =================================================================== */

type MarathonClient struct {
	/* a channel to receive updates from the endpoint from */
	update_channel chan bool
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
	service.update_channel   = make(chan bool, 5)
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
		/* step: we retrieve the marathon application */
		application, err := marathon.Application(name)
		if err != nil {
			glog.Errorf("Failed to retrieve a list of tasks for application: %s, error: %s", service.ID, err)
			return nil, err
		}

		/* step: does the application have any active tasks ? */
		if len(application.Tasks) <= 0 {
			glog.V(3).Infof("The Marathon application: %s doesn't have any endpoints", application.ID)
			return make([]Endpoint,0), nil
		}

		/* step: iterate the tasks and build the endpoints */
		endpoints, err := r.GetEndpointsFromApplication(application, service)
		if err != nil {
			glog.Errorf("Failed to extract the endpoints for Marathon application: %s, error: %s", application.ID, err)
			return nil, err
		}
		return endpoints, nil
	}
}

func (r *MarathonClient) GetEndpointsFromApplication(application Application, service *services.Service) ([]Endpoint, error) {

	/* step: we get the port index of our service from the port mappings */
	port_index, err := r.GetServicePortFromApplication(service, application)
	if err != nil {
		glog.Errorf("Failed to retrieve the port mapping index for applcation: %s, service: %s", application, service)
		return nil, err
	}

	/* step: check if we have any health check for this service */
	service_has_checks := false
	if application.HealthChecks != nil {
		for _, health := range application.HealthChecks {
			/* check: is it related to us? */
			if health.PortIndex == port_index {
				service_has_checks = true
				break
			}
		}
	}

	/*  notes: does the application have health checks? One thing i've noticed (and this could be a misconfiguration by me, but) is
		it can take a number of seconds to minutes for health checks to become active. While the health isn't yet active, the
		HealthCheckResult in the task json is missing, not false. So we can be in a situation where endpoints have been added, they haven't
		been checked yet, but the task health check status is missing.

			- check if the 'application' NOT just the task has a health check as its related to our service port
			- if yes, but the HealthCheckResult is missing or empty we have to assume the health check hasn't been added / processed yet.
			  Either way the endpoint hasn't been validated as 'passed' and so we must remove it from the active endpoints
	*/

	/* step: we iterate the tasks and extract the ports */
	endpoints := make([]Endpoint,0)
	for _, task := range application.Tasks {
		/* check: if the application has checks, but the task does not - it's not been validated yet */
		if service_has_checks && task.HealthCheckResult == nil {
			glog.V(4).Infof("The health for application: %s, task: %s:%d hasn't yet been performed, excluding from endpoints",
				application.ID, task.Host, service.Port)
		} else if service_has_checks && task.HealthCheckResult != nil {
			/* step: we have to iterate the health checks
				- find anyone of them where the Alive is false
				- check if the port index is related to the service we are grabbing endpoints for
				- and if so, exclude the endpoint from our list;
				  : @@CHOICE we could remove the endpoint, regardless of service??
			*/
			if task.HealthCheckResult == nil {
				glog.V(4).Infof("The health check for application: %s, task: %s:%d is missing", application.ID,
					task.Host, service.Port)
			} else {
				if len(task.HealthCheckResult) < port_index {
					glog.V(5).Infof("The health checks performed for application: %s does not have our service: %s yet",
						application, service)
				} else {
					health_check := task.HealthCheckResult[port_index]
					if !health_check.Alive {
						glog.V(4).Infof("Service: %s, endpoint: %s:%d health check not passed",
							service, task.Host, service.Port)
					} else {
						endpoints = append(endpoints, Endpoint(fmt.Sprintf("%s:%d", task.Host, task.Ports[port_index])))
					}
				}
			}
		} else {
			/* step: else we can simply add it to the list */
			endpoints = append(endpoints, Endpoint(fmt.Sprintf("%s:%d", task.Host, task.Ports[port_index])))
		}
	}
	glog.V(3).Infof("Found %d endpoints in marathon application: %s, service port: %d, endpoints: %v",
		len(endpoints), application.ID, service.Port, endpoints)
	return endpoints, nil
}

func (r *MarathonClient) GetServicePortFromApplication(service *services.Service, application Application) (int, error) {
	port_index := -1
	for index, port_mapping := range application.Container.Docker.PortMappings {
		if port_mapping.ContainerPort == service.Port {
			port_index = index
		}
	}
	if port_index < 0 {
		return 0, errors.New("Unable to find service port for service " + service.String())
	}
	return port_index, nil
}

func (r *MarathonClient) Watch(service *services.Service) (EndpointEventChannel, error) {
	/* channel to send back events to the endpoints store */
	endpointUpdateChannel := make(EndpointEventChannel, 5)
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
				case <- r.update_channel:
					endpointUpdateChannel <- EndpointEvent{string(service.ID),ENDPOINT_CHANGED,*service}
				case <- r.shutdown_channel:
					marathon.Remove(name, port, r.update_channel)
				}
			}
		}()
	}
	return endpointUpdateChannel, nil
}

func (r *MarathonClient) Close() {
	/* step: we need to remove our self from listening to the event from the marathon endpoint service */
	r.shutdown_channel <- true
}

func (r *MarathonClient) ServiceID(service_id string) (string, int, error) {
	if elements := MarathonServiceRegex.FindAllStringSubmatch(service_id, -1); len(elements) > 0 {
		section := elements[0]
		service_name := section[1]
		service_port, _ := strconv.Atoi(section[2])
		return service_name, service_port, nil
	} else {
		glog.Errorf("The service definition for service: %s, when using marathon as a provider must have a service port", service_id)
		return "", 0, errors.New("The service definition is invalid, please check documentation regarding marathon provider")
	}
}
