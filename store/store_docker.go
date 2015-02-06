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

package store

import (
	"errors"
	"flag"
	"regexp"
	"strings"
	"sync"

	"github.com/gambol99/embassy/config"
	docker "github.com/gambol99/go-dockerclient"
	"github.com/golang/glog"
)

const (
	DOCKER_START            = "start"
	DOCKER_DIE              = "die"
	DOCKER_CREATED          = "created"
	DOCKER_DESTROY          = "destroy"
	DOCKER_CONTAINER_PREFIX = "container:"
	DEFAULT_DOCKER_SOCKET   = "unix:///var/run/docker.sock"
)

var (
	docker_socket *string
)

func init() {
	docker_socket  = flag.String("docker", DEFAULT_DOCKER_SOCKET, "the location of the docker socket")
}

type DockerServiceStore struct {
	/* docker api client */
	Docker *docker.Client
	/* map of container id to definition */
	ServiceMap
}

func AddDockerServiceStore() (ServiceProvider, error) {
	if docker_store, err := NewDockerServiceStore(); err != nil {
		glog.Errorf("Unable to create the docker service store, error: %s", err)
		return nil, err
	} else {
		return docker_store, nil
	}
}

type ServiceMap struct {
	/* the lock for the service map */
	sync.Mutex
	/* a map containing the services - container => definition */
	Services map[string][]DefinitionEvent
}

func (r *ServiceMap) Add(containerID string, definitions []DefinitionEvent) {
	r.Lock()
	defer r.Unlock()
	if r.Services == nil {
		r.Services = make(map[string][]DefinitionEvent)
	}
	r.Services[containerID] = definitions
}

func (r *ServiceMap) Remove(containerID string) {
	r.Lock()
	defer r.Unlock()
	delete(r.Services, containerID)
}

func (r *ServiceMap) Has(containerId string) ([]DefinitionEvent, bool) {
	r.Lock()
	defer r.Unlock()
	if definitions, found := r.Services[containerId]; found {
		return definitions, true
	}
	return nil, false
}

func NewDockerServiceStore() (ServiceProvider, error) {
	/* step: we create a docker client */
	glog.V(3).Infof("Creating docker client api, socket: %s", *docker_socket)

	/* step: create a docker client */
	client, err := docker.NewClient(*docker_socket)
	if err != nil {
		glog.Errorf("Unable to create a docker client, error: %s", err)
		return nil, err
	}
	docker_store := new(DockerServiceStore)
	docker_store.Docker = client
	return docker_store, nil
}

func (r *DockerServiceStore) StreamServices(channel BackendServiceChannel) error {
	glog.V(6).Infof("Starting the docker backend service discovery stream")
	/* step: before we stream the services take the time to lookup for containers already running and find the links */
	r.LookupRunningContainers(channel)

	/* channel to receive events */
	docker_events := make(chan *docker.APIEvents)
	go func() {
		/* step: add our channel as an event listener for docker events */
		if err := r.Docker.AddEventListener(docker_events); err != nil {
			glog.Fatalf("Unable to register docker events listener, error: %s", err)
			return
		}
		/* step: start the event loop and wait for docker events */
		glog.V(5).Infof("Entering into the docker events loop")

		for {
			select {
			case event := <-docker_events:
				glog.V(4).Infof("Received docker event status: %s, id: %s", event.Status, event.ID)
				switch event.Status {
				case DOCKER_START:
					go r.ProcessDockerCreation(event.ID, channel)
				case DOCKER_DESTROY:
					go r.ProcessDockerDestroy(event.ID, channel)
				}
			}
		}
		glog.Errorf("Exitting the docker events loop")
	}()
	return nil
}

func (r *DockerServiceStore) LookupRunningContainers(channel BackendServiceChannel) error {
	glog.Infof("Looking for any container already running and checking for services")
	if containers, err := r.Docker.ListContainers(docker.ListContainersOptions{}); err == nil {
		/* step: iterate the containers and look for services */
		for _, container := range containers {
			r.ProcessDockerCreation(container.ID, channel)
		}
	} else {
		glog.Errorf("Failed to list the currently running container, error: %s", err)
		return err
	}
	return nil
}

func (r *DockerServiceStore) ProcessDockerCreation(containerID string, channel BackendServiceChannel) error {
	/* step: inspect the service of the container */
	definitions, err := r.InspectContainerServices(containerID)
	if err != nil {
		glog.Errorf("Unable to inspect container: %s for services, error: %s", containerID[:12], err)
		return err
	}
	glog.V(4).Infof("Container: %s, services found: %d", containerID[:12], len(definitions))
	/* step: add the container to the service map */
	r.Add(containerID, definitions)
	/* step: push the service */
	r.PushServices(channel, definitions, DEFINITION_SERVICE_ADDED)
	glog.V(4).Infof("Successfully added services from container: %s", containerID[:12])
	return nil
}

func (r *DockerServiceStore) ProcessDockerDestroy(containerID string, channel BackendServiceChannel) error {
	glog.V(4).Infof("Docker destruction event, container: %s", containerID[:12])
	if definitions, found := r.Has(containerID); found {
		glog.V(4).Infof("Found %d definitions for container: %s", len(definitions), containerID[:12])
		r.PushServices(channel, definitions, DEFINITION_SERVICE_REMOVED)
		r.Remove(containerID)
		glog.V(4).Infof("Successfully removed services from container: %s", containerID[:12])
	} else {
		glog.V(4).Infof("Failed to find any defintitions from container: %s", containerID[:12])
	}
	return nil
}

func (r *DockerServiceStore) PushServices(channel BackendServiceChannel, definitions []DefinitionEvent, operation DefinitionOperation) {
	for _, definition := range definitions {
		definition.Operation = operation
		channel <- definition
	}
}

func (r *DockerServiceStore) GetContainer(containerID string) (container *docker.Container, err error) {
	glog.V(5).Infof("Grabbing the container: %s from docker api", containerID)
	container, err = r.Docker.InspectContainer(containerID)
	return
}

func (r *DockerServiceStore) InspectContainerServices(containerID string) ([]DefinitionEvent, error) {
	definitions := make([]DefinitionEvent, 0)
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
		/* step: if we */
		if r.IsBackendService(key) {
			glog.V(3).Infof("Found backend request in container: %s, service: %s", containerID, value)
			/* step: create a backend definition and append to list */
			var definition DefinitionEvent
			definition.Name = key
			definition.SourceAddress = source_address
			definition.Definition = value
			definitions = append(definitions, definition)
		}
	}
	return definitions, nil
}

/*
	A container is assumed to associated to the proxy if they has the same ip address as us or
	the container is running in network mode container and we are the container
*/
func (r DockerServiceStore) GetContainerIPAddress(container *docker.Container) (string, error) {
	/* step: does the docker have an ip address */
	if source_address := container.NetworkSettings.IPAddress; source_address != "" {
		glog.V(4).Infof("Container: %s, source ip address: %s", container.ID[:12], source_address)
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

func (r DockerServiceStore) IsBackendService(key string) (found bool) {
	found, _ = regexp.MatchString("^+" + config.Options.Service_prefix, key)
	return
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
			glog.V(4).Infof("Invalid environment variable: %s, skipping", kv)
		}
	}
	return environment, nil
}
