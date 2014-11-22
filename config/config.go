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
	"net/url"

	"github.com/gambol99/embassy/utils"
	"github.com/golang/glog"
)

var (
	socket, discovery, iface, fixed_backend *string
)

func init() {
	socket = flag.String("docker", "unix://var/run/docker.sock", "the location of the docker socket")
	discovery = flag.String("discovery", "etcd://localhost:4001", "the discovery backend to pull the services from")
	iface = flag.String("interface", "eth0", "the interface to take the proxy address from")
	fixed_backend = flag.String("backend", "", "allow you specifiy a fixed backend service")
}

type Configuration struct {
	DockerSocket  string
	DiscoveryURI  string
	FixedBackend  string
	BackendPrefix string
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

func NewConfiguration() *Configuration {
	configuration := new(Configuration)
	configuration.DiscoveryURI = *discovery
	configuration.DockerSocket = *socket
	configuration.BackendPrefix = "BACKEND_"
	configuration.Interface = *iface

	configuration.FixedBackend = *fixed_backend
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
