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
	"os"
	"regexp"
	"strings"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/gambol99/embassy/config"
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
	Config  *config.Configuration
	Events  DockerEventsChannel /* docker events channel */
	Updates ServiceStoreChannel /* service request are passed into this channel */
}

func NewDockerServiceStore(config *config.Configuration, channel ServiceStoreChannel) (ServiceStore, error) {
	/* step: we create a docker client */
	glog.V(3).Infof("Creating docker client api, socket: %s", config.DockerSocket)
	/* step: validate the socket */
	if err := ValidateDockerSocket(config.DockerSocket); err != nil {
		return nil, err
	}
	/* step: create a docker client */
	client, err := docker.NewClient(config.DockerSocket);
	if err != nil {
		glog.Errorf("Unable to create a docker client, error: %s", err)
		return nil, err
	}
	return &DockerServiceStore{client, config, nil, channel}, nil
}

func (r *DockerServiceStore) DiscoverServices() error {
	glog.V(1).Info("Starting the docker backend service discovery stream")
	if err := r.AddDockerEventListener(); err != nil {
		glog.Errorf("Unable to add our docker client as an event listener, error:", err)
		return err
	}
	/* step: create a goroutine to listen to the events */
	go func(docker *DockerServiceStore) {
		glog.V(4).Infof("Entering into the docker events loop")
		for event := range docker.Events {
			glog.V(5).Infof("Received docker event: %s passing to handler", event)
			r.DockerEventUpdate(event.Status, event.ID)
			glog.V(5).Infof("Looping around for next event")
		}
	}(r)
	return nil
}

func (r *DockerServiceStore) DockerEventUpdate(eventType, containerId string) (err error) {
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
	glog.V(5).Infof("Adding the docker event listen to our channel")
	/* step: create a channel for docker events */
	r.Events = make(DockerEventsChannel)
	/* step: add our channel as an event listener for docker events */
	if err = r.Docker.AddEventListener(r.Events); err != nil {
		glog.Errorf("Unable to register docker events listener, error: %s", err)
		return
	}
	glog.V(5).Infof("Successfully added the docker event handler")
	return
}

func (r DockerServiceStore) InspectContainerServices(containerId string) (definitions []Service, err error) {
	definitions = make([]Service, 0)
	/* step: grab the container */
	if container, err := r.Docker.InspectContainer(containerId); err == nil {
		/* step: we are ONLY concerned with containers that are linked to this proxy */
		if associated  := r.IsAssociated(container); associated {
			glog.V(0).Infof("Container: %s linked to proxy, inspecting the services", containerId)
			if environment, err := ContainerEnvironment(container.Config.Env); err == nil {
				/* step; scan the runtime variables for backend links */
				for key, value := range environment {
					glog.V(5).Infof("Runtime vars, key: %s value: %s", key, value)
					if r.IsBackendService(key, value) {
						glog.V(2).Infof("Found backend request in container: %s, service: %s", containerId, value)
						/* step: create a backend defintion, validate and convert to service definition */
						var definition BackendDefiniton
						definition.Name = key
						definition.Definition = value
						/* check: is the definition valid */
						service, err := definition.GetService()
						if err != nil {
							glog.Errorf("Invalid service definition, error: %s", err)
							continue
						}
						/* step: else we add to the list */
						definitions = append(definitions, service)
					} else {
						glog.V(6).Infof("Runtime; %s = %s is not a backend service request", key, value)
					}
					if err != nil {
						glog.Errorf("Invalid service definition found in container: %s, service: %s, error: %s", containerId, value, err)
					}
				}
			}
		} else {
			glog.V(0).Info("Container: %s is not linked to our proxy, skipping", containerId)
		}
	}
	return
}

const (
	DOCKER_NETWORK_CONTAINER_PREFIX = "container:"
)

/*
	A container is assumed to associated to the proxy if they has the same ip address as us or
	the container is running in network mode container and we are the container
 */
func (r DockerServiceStore) IsAssociated(container *docker.Container) bool {
	/* step: does the docker have an ip address */
	if docker_ipaddress := GetDockerIPAddress(container); docker_ipaddress != "" {
		if docker_ipaddress == r.Config.IPAddress {
			glog.V(2).Infof("Container: %s and proxy have the same ip address", container.ID )
			return true
		}
	} else {
		/* step: is the container running in NetworkMode = container */
		if network_mode := container.HostConfig.NetworkMode; strings.HasPrefix(network_mode, DOCKER_NETWORK_CONTAINER_PREFIX ) {
			// lets get the container this container is linked to and see if its us
			container_name := strings.TrimPrefix(network_mode, DOCKER_NETWORK_CONTAINER_PREFIX )
			glog.V(5).Infof("Container: %s running net:container mode, mapping into container: %s", container.ID, container_name )
			if container_name == r.Config.HostName {
				return true
			}
		} else {
			glog.Errorf("The container doesnt have an ip address and isn't running network mode: container")
		}
	}
	return false
}

func (r DockerServiceStore) IsBackendService(key, value string) (found bool) {
	found, _ = regexp.MatchString(r.Config.BackendPrefix, key)
	return
}

func ValidateDockerSocket(socket string) error {
	glog.V(5).Infof("Validating the docker socket: %s", socket)
	if match, _ := regexp.MatchString("^unix://", socket); !match {
		glog.Errorf("The docker socket: %s should start with unix://", socket)
		return errors.New("Invalid docker socket")
	}
	filename := strings.TrimPrefix(socket, "unix:/")
	glog.V(5).Infof("Looking for docker socket: %s", filename)
	if filestat, err := os.Stat(filename); err != nil {
		glog.Errorf("The docker socket: %s does not exists", socket)
		return errors.New("The docker socket does not exist")
	} else if filestat.Mode() == os.ModeSocket {
		glog.Errorf("The docker socket: %s is not a unix socket, please check", socket)
		return errors.New("The docker socket is not a named unix socket")
	}
	return nil
}

func GetDockerIPAddress(container *docker.Container) string {
	return container.NetworkSettings.IPAddress
}

func ContainerStringID(containerId string) string {
	return containerId[:12]
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
