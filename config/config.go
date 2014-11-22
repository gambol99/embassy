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
	cfg_socket, cfg_discovery, cfg_iface, cfg_fixed_backend *string
	cfg_association *bool
)

func init() {
	cfg_socket = flag.String("docker", "unix://var/run/docker.sock", "the location of the docker socket")
	cfg_discovery = flag.String("discovery", "etcd://localhost:4001", "the discovery backend to pull the services from")
	cfg_iface = flag.String("interface", "eth0", "the interface to take the proxy address from")
	cfg_fixed_backend = flag.String("backend", "", "allow you specifiy a fixed backend service")
	cfg_association = flag.Bool("association",true,"whether or not to check association of the container to the proxy" )
}

type Configuration struct {
	DockerSocket  string
	DiscoveryURI  string
	FixedBackend  string
	BackendPrefix string
	Association   bool
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
	configuration.DiscoveryURI = *cfg_discovery
	configuration.DockerSocket = *cfg_socket
	configuration.BackendPrefix = "BACKEND_"
	configuration.Interface = *cfg_iface
	configuration.Association = *cfg_association
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
