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

package config

import (
	"flag"
	"fmt"
	"net/url"

	"github.com/gambol99/embassy/utils"
	"github.com/golang/glog"
)

var (
	cfg_socket, cfg_discovery, cfg_iface, cfg_fixed_backend *string
	cfg_association                                         *bool
	cfg_proxy_port                                          *int
)

const (
	DEFAULT_PROXY_PORT     = 9999
	DEFAULT_INTERFACE      = "eth0"
	DEFAULT_DOCKER_SOCKET  = "unix://var/run/docker.sock"
	DEFAULT_DISCOVERY_URI  = "etcd://localhost:4001"
	DEFAULT_FIXED_BACKEND  = ""
	DEFAULT_SERVICE_PREFIX = "BACKEND_"
)

func init() {
	cfg_socket = flag.String("docker", DEFAULT_DOCKER_SOCKET, "the location of the docker socket")
	cfg_discovery = flag.String("discovery", DEFAULT_DISCOVERY_URI, "the discovery backend to pull the services from")
	cfg_iface = flag.String("interface", DEFAULT_INTERFACE, "the interface to take the proxy address from")
	cfg_fixed_backend = flag.String("backend", DEFAULT_FIXED_BACKEND, "allow you specifiy a fixed backend service")
	cfg_proxy_port = flag.Int("port", DEFAULT_PROXY_PORT, "the tcp port which the proxy should listen on")
}

type Configuration struct {
	DockerSocket  string
	DiscoveryURI  string
	FixedBackend  string
	BackendPrefix string
	ProxyPort     int
	IPAddress     string
	Interface     string
	HostName      string
}

func (s Configuration) GetDiscoveryURI() *url.URL {
	uri, _ := url.Parse(s.DiscoveryURI)
	return uri
}

func (s Configuration) ValidConfiguration() error {
	return nil
}

func (s Configuration) String() string {
	return fmt.Sprintf("socket: %s, discovery: %s, proxy: (%s) %s:%d", s.DockerSocket, s.DiscoveryURI,
		s.Interface, s.IPAddress, s.ProxyPort)
}

func NewConfiguration() *Configuration {
	configuration := new(Configuration)
	configuration.DiscoveryURI = *cfg_discovery
	configuration.DockerSocket = *cfg_socket
	configuration.BackendPrefix = DEFAULT_SERVICE_PREFIX
	configuration.ProxyPort = *cfg_proxy_port
	configuration.Interface = *cfg_iface
	configuration.FixedBackend = *cfg_fixed_backend
	hostname, err := utils.GetHostname()
	if err != nil {
		glog.Errorf("Unable to get the hostname of the box, error: %s", err)
		return nil
	}
	configuration.HostName = hostname
	ipaddress, err := utils.GetLocalIPAddress(configuration.Interface)
	if err != nil {
		glog.Error("Unable to get the local ip address from interface: %s, error: %s",
			configuration.Interface, err)
		return nil
	}
	configuration.IPAddress = ipaddress
	return configuration
}
