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
	"regexp"
	"strings"
	"sync"

	"github.com/gambol99/embassy/config"
	"github.com/gambol99/embassy/proxy/services"
	"github.com/gambol99/embassy/utils"
	docker "github.com/gambol99/go-dockerclient"

	"github.com/golang/glog"
)

const (
	DOCKER_START            = "start"
	DOCKER_DESTROY          = "destroy"
	DOCKER_CONTAINER_PREFIX = "container:"
)

type DockerServiceStore struct {
	sync.RWMutex
	/* map of container ids we've seen */
	seen map[string][]Definition
	/* docker api client */
	client *docker.Client
	/* a channel for send our updates */
	updates services.ServicesChannel
	/* a shutdown channel */
	shutdown utils.ShutdownSignalChannel
}

func NewDockerServiceStore() (services.ServiceStore, error) {
	glog.V(3).Infof("Creating docker client api, socket: %s", config.Options.Socket)
	client, err := docker.NewClient(config.Options.Socket)
	if err != nil {
		return nil, err
	}
	docker_store := new(DockerServiceStore)
	docker_store.client = client
	docker_store.shutdown = make(utils.ShutdownSignalChannel)
	docker_store.seen = make(map[string][]Definition, 0)
	return docker_store, nil
}

func (r *DockerServiceStore) Close() error {
	r.shutdown <- true
	return nil
}

func (r *DockerServiceStore) StreamServices(channel services.ServicesChannel) error {

	// step: before we stream the services take the time to lookup for containers already running and find the links
	if err := r.lookupRunningContainers(channel); err != nil {
		glog.Errorf("Failed to perform a lookup of running container, error: %s", err)
	}

	// step: add our channel as an event listener for docker events
	events := make(chan *docker.APIEvents)
	if err := r.client.AddEventListener(events); err != nil {
		glog.Errorf("Unable to register docker events listener, error: %s", err)
		return err
	}

	go func() {
		defer close(channel)
		for {
			select {
			// Request to shutdown the service
			case <-r.shutdown:
				glog.Infof("Received a shutdown signal, closing off resource now")
				// step: remove our self as a listener
				if err := r.client.RemoveEventListener(events); err != nil {
					glog.Errorf("Failed to remove the events listener, error: %s", err)
				}
				break

			// We have an event from docker
			case event := <-events:
				glog.V(5).Infof("Received docker event status: %s, id: %s", event.Status, event.ID)
				switch event.Status {
				case DOCKER_START:
					r.processDockerCreation(event.ID, channel)
				case DOCKER_DESTROY:
					r.processDockerDestroy(event.ID, channel)
				}
			}
		}
	}()
	return nil
}

// --- Docker Related ---

func (r *DockerServiceStore) processDockerCreation(containerID string, channel services.ServicesChannel) error {
	glog.V(5).Infof("Docker creation event, container: %s", containerID[:12])

	// step: inspect the service of the container
	definitions, err := r.inspectContainerServices(containerID)
	if err != nil {
		glog.Errorf("Unable to inspect container: %s for services, error: %s", containerID[:12], err)
		return err
	}

	// step: add the container to the service map
	r.addContainer(containerID, definitions)

	// step: push the service
	pushServices(channel, definitions, true)
	return nil

}

func (r *DockerServiceStore) processDockerDestroy(containerID string, channel services.ServicesChannel) error {
	glog.V(5).Infof("Docker destruction event, container: %s", containerID[:12])

	// Check: has the service already been seen?
	definitions, found := r.seenContainer(containerID)
	if !found {
		glog.V(4).Infof("Failed to find any definitions from container: %s", containerID[:12])
		return nil
	}
	// step: remove the definition from the seen map
	r.removeContainer(containerID)

	// step: push the service event up to the listener
	pushServices(channel, definitions, false)
	return nil
}

func (r *DockerServiceStore) lookupRunningContainers(channel services.ServicesChannel) error {
	glog.V(4).Infof("Looking for any container already running and checking for services")
	containers, err := r.client.ListContainers(docker.ListContainersOptions{})
	if err != nil {
		return err
	}
	// step: iterate the containers and look for services
	for _, container := range containers {
		r.processDockerCreation(container.ID, channel)
	}
	return nil
}

func (r *DockerServiceStore) inspectContainerServices(containerID string) ([]Definition, error) {

	definitions := make([]Definition, 0)

	// step: get the container configuration
	container, err := r.client.InspectContainer(containerID)
	if err != nil {
		glog.Errorf("Failed to retrieve the container config from api, container: %s, error: %s", containerID, err)
		return nil, err
	}

	// step: grab the source ip address of the container
	source_address, err := r.getContainerIPAddress(container)
	if err != nil {
		glog.Errorf("Failed to get the container ip address, container: %s, error: %s", containerID[:12], err)
		return nil, err
	}

	// step: build the environment map of the container
	environment, err := r.containerEnvironment(container.Config.Env)
	if err != nil {
		glog.Errorf("Unable to retrieve the environment fron the container: %s, error: %s", containerID[:12], err)
		return nil, err
	}

	// step; scan the runtime variables for backend links
	for key, value := range environment {
		if r.isBackendService(key) {
			glog.V(6).Infof("Found backend request in container: %s, service: %s", containerID, value)
			definition := Definition{
				Name:          key,
				SourceAddress: source_address,
				Definition:    value,
			}
			if !definition.IsValid() {
				glog.Errorf("Invalid service definition: %s, skipping service binding", definition)
				continue
			}

			definitions = append(definitions, definition)
		}
	}
	glog.V(4).Infof("Container: %s, services found: %d", containerID[:12], len(definitions))

	return definitions, nil
}

//
//	A container is assumed to associated to the proxy if they has the same ip address as us or
//	the container is running in network mode container and we are the container
//
func (r DockerServiceStore) getContainerIPAddress(container *docker.Container) (string, error) {

	// step: does the docker have an ip address
	if source_address := container.NetworkSettings.IPAddress; source_address != "" {
		glog.V(4).Infof("Container: %s, source ip address: %s", container.ID[:12], source_address)
		return source_address, nil
	} else {

		/* step: is the container running in NetworkMode = container */
		if network_mode := container.HostConfig.NetworkMode; strings.HasPrefix(network_mode, DOCKER_CONTAINER_PREFIX) {

			/* step: get the container id of the network container */
			container_name := strings.TrimPrefix(network_mode, DOCKER_CONTAINER_PREFIX)
			glog.V(5).Infof("Container: %s running net:container mode, mapping into container: %s", container.ID[:12], container_name)

			/* step: grab the actual network container */
			network_container, err := r.client.InspectContainer(container_name)
			if err != nil {
				glog.Errorf("Failed to retrieve the network container: %s for container: %s, error: %s", container_name, container.ID[:12], err)
				return "", err
			}

			/* step: take that and grab the ip address from it */
			source_address, err := r.getContainerIPAddress(network_container)
			if err != nil {
				glog.Error("Failed to get the ip address of the network container: %s, error: %s", container_name, err)
				return "", err
			}
			return source_address, nil
		} else {
			return "", errors.New("Failed to retrieve the ip address of the container, doesn't appear to have one")
		}
	}
}

func (r DockerServiceStore) isBackendService(key string) (found bool) {
	found, _ = regexp.MatchString("^+"+config.Options.Service_prefix, key)
	return
}

// Method: take the environment variables (an error of key=value) and convert them to a map */
func (r DockerServiceStore) containerEnvironment(variables []string) (map[string]string, error) {
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

func (r *DockerServiceStore) seenContainer(containerID string) ([]Definition, bool) {
	r.RLock()
	defer r.RUnlock()
	if defs, found := r.seen[containerID]; found {
		return defs, true
	}
	return nil, false
}

func (r *DockerServiceStore) addContainer(containerID string, definitions []Definition) {
	r.Lock()
	defer r.Unlock()
	r.seen[containerID] = definitions
}

func (r *DockerServiceStore) removeContainer(containerID string) {
	r.Lock()
	defer r.Unlock()
	delete(r.seen, containerID)
}
