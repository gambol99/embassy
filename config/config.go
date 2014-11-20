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

package config

import (
	"flag"
	"net/url"
	"os"
)

var (
	socket, discovery, iface, fixed *string
)

func init() {
	socket = flag.String("docker", "unix://var/run/docker.sock", "the location of the docker socket")
	discovery = flag.String("discovery", "etcd://localhost:4001", "the discovery backend to pull the services from")
	iface = flag.String("interface", "eth0", "the interface to take the proxy address from")
	fixed_backend = flag.String("backend", "", "allow you specifiy a fixed backend service")
}

type ServiceOptions map[string]string

func (s ServiceOptions) Get(key string) (value string, found bool) {
	value, found = s[key]
	return
}

func (s ServiceOptions) Has(key string) bool {
	if _, found := s[key]; found {
		return true
	}
	return false
}

type Configuration struct {
	DockerSocket  string
	DiscoveryURI  string
	FixedBackend  string
	BackendPrefix string
	Interface     string
}

func (s Configuration) GetDiscoveryURI() *url.URL {
	uri, _ := url.Parse(s.DiscoveryURI)
	return uri
}

func (s Configuration) ValidConfiguration() error {
	return nil
}

func NewConfiguration() *Configuration {
	configuration := new(Configuration)
	configuration.DiscoveryURI = *discovery
	configuration.DockerSocket = *socket
	configuration.BackendPrefix = "BACKEND_"
	configuration.Interface = *iface
	configuration.FixedBackend = *fixed_backend
	/* step: check if the command line has a fixed backend */
	if length := len(os.Args); length == 2 {
		configuration.FixedBackend = os.Args[1]
	}
	return configuration
}

func ProgName() string {
	return os.Args[0]
}
