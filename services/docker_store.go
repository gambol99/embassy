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
package services

import (
	"errors"
	"regexp"
	"strings"

	"github.com/fsouza/go-dockerclient"
	"github.com/gambol99/embassy/config"
	"github.com/gambol99/embassy/utils"
	"github.com/golang/glog"
)

const (
	DOCKER_EVENT_START   = "start"
	DOCKER_EVENT_DIE     = "die"
	DOCKER_EVENT_CREATED = "created"
	DOCKER_EVENT_DESTROY = "destroy"
)

type DockerEventsChannel chan *docker.APIEvents

type DockerServiceStore struct {
	Docker  *docker.Client /* docker api client */
	Address string
	Config  *config.Configuration
	Events  DockerEventsChannel /* docker events channel */
	Updates ServiceStoreChannel /* service request are passed into this channel */
}

func NewDockerServiceStore(config *config.Configuration, channel ServiceStoreChannel) (ServiceStore, error) {
	/* step: we create a docker client */
	if client, err := docker.NewClient(config.DockerSocket); err != nil {
		glog.Errorf("Unable to create a docker client, error: %s", err)
		return nil, err
	} else {
		/* step: we need the ip address of the proxy */
		if ipaddress, err := utils.GetLocalIPAddress("eth0"); err != nil {
			glog.Errorf("Unable to get the ip address of the proxy, error: %s", err)
			return nil, err
		} else {
			/* step: create the service provider */
			service := &DockerServiceStore{client, ipaddress, config, nil, channel}

			/* step: kick off the discovery loop */
			if err := service.DiscoverServices(); err != nil {
				glog.Errorf("Unable to start the docker discovery loop, error: %s", err)
				return nil, err
			}
			return service, nil
		}
	}
}

func (r DockerServiceStore) DiscoverServices() error {
	glog.V(1).Info("Starting the docker backend service discovery stream")
	if err := r.AddDockerEventListener(); err != nil {
		return err
	}
	/* step: create a goroutine to listen to the events */
	go func() {
		for {
			event := <-r.Events
			r.DockerEventUpdate(event.Status, event.ID)
		}
	}()
	return nil
}

func (r DockerServiceStore) DockerEventUpdate(eventType, containerId string) (err error) {
	glog.V(2).Infof("Recieved docker event, status: %s, container: %s", eventType, containerId)
	switch eventType {
	case DOCKER_EVENT_START:
		/* step: inspect the container for services */
		go func(id string) {
			if services, err := r.InspectContainerServices(id); err != nil {
				glog.Errorf("Unable to inspect container: %s for services, error: %s", id, err)
			} else {
				if len(services) <= 0 {
					glog.V(2).Infof("No backend service requests in container: %s, skipping", id)
				} else {
					/* step: we found services in the container, lets push them */
					for _, service := range services {
						glog.V(3).Infof("Pushing service request to events channel: %s", service)
						r.Updates <- service
					}
				}
			}
		}(containerId)
	case DOCKER_EVENT_DIE:
	default:
	}
	return
}

func (r *DockerServiceStore) AddDockerEventListener() (err error) {
	/* step: create a channel for docker events */
	r.Events = make(chan *docker.APIEvents)
	/* step: add our channel as an event listener for docker events */
	if err = r.Docker.AddEventListener(r.Events); err != nil {
		glog.Error("Unable to register docker events listener, error: %s", err)
		return
	}
	return
}

func (r DockerServiceStore) InspectContainerServices(containerId string) (definitions []Service, err error) {
	definitions = make([]Service, 0)
	/* step: grab the container */
	if container, err := r.Docker.InspectContainer(containerId); err == nil {
		/* step: we are ONLY concerned with containers that are linked to this proxy */
		if ipaddress, err := GetDockerIPAddress(container); err != nil {
			glog.Errorf("Unable to get the container ip address, skipping the container for now")
		} else {
			/* check: is the container associated to ourself */
			if ipaddress != r.Address {
				glog.Infof("The container: %s is not linked to proxy, refusing to inspect services", containerId)
				return definitions, nil
			}
			glog.V(3).Infof("container: %s linked to proxy, inspecting the services", containerId)
			if environment, err := ContainerEnvironment(container.Config.Env); err == nil {
				/* step; scan the runtime variables for backend links */
				for key, value := range environment {
					if r.IsBackendService(key, value) {
						/* step: create a backend defintion, validate and convert to service definition */
						var definition BackendDefiniton
						definition.Name = key
						definition.Definition = value
						/* check: is the definition valid */
						if service, err := definition.GetService(); err == nil {
							definitions = append(definitions, service)
						}
					}
					if err != nil {
						glog.Errorf("Invalid service definition found in container: %s, service: %s, error: %s", containerId, value, err)
					}
				}
			}
		}
	}
	return
}

func (r DockerServiceStore) IsBackendService(key, value string) (found bool) {
	found, _ = regexp.MatchString(r.Config.BackendPrefix, key)
	return
}

func GetDockerIPAddress(container *docker.Container) (string, error) {
	if address := container.NetworkSettings.IPAddress; address == "" {
		glog.Infof("The container: %s does not have an ipaddress", container.ID)
		return "", errors.New("The container does not have an ipaddress")
	} else {
		return address, nil
	}
}

/*
  Method: take the environment variables (an error of key=value) and convert them to a map
*/
func ContainerEnvironment(env []string) (map[string]string, error) {
	environment := make(map[string]string, 0)
	for _, kv := range env {
		if found, _ := regexp.MatchString(`^(.*)=(.*)$`, kv); found {
			elements := strings.SplitN(kv, "=", 2)
			environment[elements[0]] = elements[1]
		} else {
			glog.V(3).Infof("Invalid environment variable: %s, skipping", kv)
		}
	}
	return environment, nil
}
