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

package services

import (
	"errors"
	"os"
	"regexp"
	"strings"
	"sync"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/gambol99/embassy/config"
	"github.com/golang/glog"
)

const (
	DOCKER_START            = "start"
	DOCKER_DIE              = "die"
	DOCKER_CREATED          = "created"
	DOCKER_DESTROY          = "destroy"
	DOCKER_CONTAINER_PREFIX = "container:"
)

type DockerEventsChannel chan *docker.APIEvents

type DockerServiceStore struct {
	sync.RWMutex
	/* docker api client */
	Docker *docker.Client
	/* the service configuraton */
	Config *config.Configuration
	/* the docker events channel */
	Events DockerEventsChannel
	/* map of container id to definition */
	Services map[string][]Definition
}

func NewDockerServiceStore(cfg *config.Configuration) (ServiceProvider, error) {
	/* step: we create a docker client */
	glog.V(3).Infof("Creating docker client api, socket: %s", cfg.DockerSocket)
	/* step: validate the socket */
	if err := ValidateDockerSocket(cfg.DockerSocket); err != nil {
		return nil, err
	}
	/* step: create a docker client */
	client, err := docker.NewClient(cfg.DockerSocket)
	if err != nil {
		glog.Errorf("Unable to create a docker client, error: %s", err)
		return nil, err
	}
	docker_store := new(DockerServiceStore)
	docker_store.Docker = client
	docker_store.Config = cfg
	docker_store.Events = make(DockerEventsChannel,5)
	docker_store.Services = make(map[string][]Definition)
	return docker_store, nil
}

func (r *DockerServiceStore) StreamServices(channel BackendServiceChannel) error {
	glog.V(6).Infof("Starting the docker backend service discovery stream")
	if err := r.AddDockerEventListener(); err != nil {
		glog.Errorf("Unable to add our docker client as an event listener, error:", err)
		return err
	}
	/* step: create a goroutine to listen to the events */
	go func() {
		glog.V(5).Infof("Entering into the docker events loop")
		/* step: before we stream the services take the time to lookup for containers already running and find the links */
		r.LookupRunningContainers(channel)
		/* step: start the event loop and wait for docker events */
		for event := range r.Events {
			glog.V(2).Infof("Received docker event: %s, container: %s", event.Status, event.ID[:12])
			switch event.Status {
			case DOCKER_START:
				go func() {
					services, err := r.InspectContainerServices(event.ID)
					if err != nil {
						glog.Errorf("Unable to inspect container: %s for services, error: %s", event.ID[:12], err)
						return
					}
					if len(services) <= 0 {
						glog.V(2).Infof("No service request found in container: %s", event.ID[:12])
						return
					}
					r.Lock()
					defer r.Unlock()
					r.Services[event.ID] = services
					for _, service := range services {
						r.PushService(channel, service, event.ID[:12])
					}
				}()
			case DOCKER_DESTROY:
				go func() {
					/* step: did the docker export a service? */
					r.Lock()
					defer r.Unlock()
					if definitions, found := r.Services[event.ID]; found {
						for _, definition := range definitions {
							definition.Operation = SERVICE_REMOVED
							r.PushService(channel, definition, event.ID[:12])

						}
					}
					delete(r.Services,event.ID)
				}()
			default:
			}
			glog.V(5).Infof("Docker event: %s, handled, looping around", event.Status)
		}
		glog.Errorf("Exitting the docker event loop")
	}()
	return nil
}

func (r *DockerServiceStore) PushService(channel BackendServiceChannel, definition Definition, containerId string) {
	glog.V(2).Infof("Pushing service: %s, container: %s ", definition, containerId)
	channel <- definition
}

func (r *DockerServiceStore) LookupRunningContainers(channel BackendServiceChannel) error {
	glog.Infof("Looking for any container already running and checking for services")
	if containers, err := r.Docker.ListContainers(docker.ListContainersOptions{}); err == nil {
		/* step: iterate the containers and look for services */
		for _, containerID := range containers {
			services, err := r.InspectContainerServices(containerID.ID)
			if err != nil {
				glog.Errorf("Unable to inspect container: %s for services, error: %s", containerID.ID[:12], err)
				continue
			}
			for _, service := range services {
				r.PushService(channel, service, containerID.ID[:12])
			}
		}
	} else {
		glog.Errorf("Failed to list the currently running container, error: %s", err)
		return err
	}
	return nil
}

func (r *DockerServiceStore) GetContainer(containerID string) (container *docker.Container, err error) {
	glog.V(5).Infof("Grabbing the container: %s from docker api", containerID)
	container, err = r.Docker.InspectContainer(containerID)
	return
}

func (r *DockerServiceStore) InspectContainerServices(containerID string) ([]Definition, error) {
	definitions := make([]Definition, 0)
	/* step: get the container config */
	container, err := r.GetContainer(containerID)
	if err != nil {
		glog.Errorf("Failed to retrieve the container config from api, container: %s, error: %s", containerID, err)
		return nil, err
	}

	/* step: grab the source ip address of the container */
	source_address, err := r.GetContainerIPAddress(container)
	if err != nil {
		glog.Errorf("Failed to get the container ip address, container: %s, error: %s", containerID[:12], err)
		return nil, err
	}

	/* step: build the environment map of the container */
	environment, err := ContainerEnvironment(container.Config.Env)
	if err != nil {
		glog.Errorf("Unable to retrieve the environment fron the container: %s, error: %s", containerID[:12], err)
		return nil, err
	}

	/* step; scan the runtime variables for backend links */
	for key, value := range environment {
		if r.IsBackendService(key, value) {
			glog.V(2).Infof("Found backend request in container: %s, service: %s", containerID, value)
			/* step: create a backend definition and append to list */
			var definition Definition
			definition.Name = key
			definition.SourceAddress = source_address
			definition.Definition = value
			definitions = append(definitions, definition)
		}
	}
	return definitions, nil
}

func (r *DockerServiceStore) AddDockerEventListener() (err error) {
	glog.V(5).Infof("Adding the docker event listen to our channel")
	/* step: add our channel as an event listener for docker events */
	if err = r.Docker.AddEventListener(r.Events); err != nil {
		glog.Errorf("Unable to register docker events listener, error: %s", err)
		return
	}
	glog.V(5).Infof("Successfully added the docker event handler")
	return
}

/*
	A container is assumed to associated to the proxy if they has the same ip address as us or
	the container is running in network mode container and we are the container
*/
func (r DockerServiceStore) GetContainerIPAddress(container *docker.Container) (string, error) {
	/* step: does the docker have an ip address */
	if source_address := container.NetworkSettings.IPAddress; source_address != "" {
		glog.V(2).Infof("Container: %s, source ip address: %s", container.ID[:12], source_address)
		return source_address, nil
	} else {
		/* step: check if the container is in NetworkMode = container and if so, grab the ip address of the container */
		/* step: is the container running in NetworkMode = container */
		if network_mode := container.HostConfig.NetworkMode; strings.HasPrefix(network_mode, DOCKER_CONTAINER_PREFIX) {
			/* step: get the container id of the network container */
			container_name := strings.TrimPrefix(network_mode, DOCKER_CONTAINER_PREFIX)
			glog.V(5).Infof("Container: %s running net:container mode, mapping into container: %s", container.ID[:12], container_name)
			/* step: grab the actual network container */
			network_container, err := r.GetContainer(container_name)
			if err != nil {
				glog.Errorf("Failed to retrieve the network container: %s for container: %s, error: %s", container_name, container.ID[:12], err)
				return "", err
			}
			/* step: take that and grab the ip address from it */
			source_address, err := r.GetContainerIPAddress(network_container)
			if err != nil {
				glog.Error("Failed to get the ip address of the network container: %s, error: %s", container_name, err)
				return "", err
			}
			return source_address, nil
		} else {
			glog.Errorf("The container doesnt have an ip address and isn't running network mode: container")
			return "", errors.New("Failed to retrieve the ip address of the container, doesn't appear to have one")
		}
	}
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

/*
  Method: take the environment variables (an error of key=value) and convert them to a map
*/
func ContainerEnvironment(variables []string) (map[string]string, error) {
	environment := make(map[string]string, 0)
	for _, kv := range variables {
		if found, _ := regexp.MatchString(`^(.*)=(.*)$`, kv); found {
			elements := strings.SplitN(kv, "=", 2)
			environment[elements[0]] = elements[1]
		} else {
			glog.V(3).Infof("Invalid environment variable: %s, skipping", kv)
		}
	}
	return environment, nil
}
