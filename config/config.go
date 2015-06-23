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
)

const (
	DEFAULT_PROXY_PORT     = 9999
	DEFAULT_INTERFACE      = "eth0"
	DEFAULT_IP_LISTEN      = ""
	DEFAULT_DISCOVERY_URI  = "consul://127.0.0.1:8500"
	DEFAULT_SERVICE_PREFIX = "BACKEND_"
	DEFAULT_FILTER_HEALTH  = true
	DEFAULT_DOCKER_SOCKET  = "unix:///var/run/docker.sock"
	DEFAULT_FORCED_RESYNC  = 120
)

var Options struct {
	/* the interface the proxy should be listening */
	Proxy_interface string
	/* the default ip address to listen on, this take priority over the interface above */
	Proxy_listen string
	/* the proxy port */
	Proxy_port int
	/* the discovery provider for service */
	Discovery_url string
	/* the service prefix */
	Service_prefix string
	/* whether or not to filter endpoints by the provider health checks */
	Filter_On_Health bool
	/* the docker socket */
	Socket string
	/* the services provider to use */
	Provider string
	/* the service options, used by the static provider */
	Services string
	/* the time in seconds to perform a forced resync of the endpoints */
	Forced_resync int
}

func init() {
	flag.StringVar(&Options.Proxy_interface, "interface", DEFAULT_INTERFACE, "the interface to take the proxy address from")
	flag.StringVar(&Options.Proxy_listen, "address", DEFAULT_IP_LISTEN, "the ip address we should be listening on (note: this takes priority over the interface option)")
	flag.IntVar(&Options.Proxy_port, "port", DEFAULT_PROXY_PORT, "the tcp port which the proxy should listen on")
	flag.StringVar(&Options.Discovery_url, "discovery", DEFAULT_DISCOVERY_URI, "the discovery backend to pull the services from")
	flag.StringVar(&Options.Service_prefix, "prefix", DEFAULT_SERVICE_PREFIX, "the prefix used to distinguish a backend service")
	flag.BoolVar(&Options.Filter_On_Health, "filter", DEFAULT_FILTER_HEALTH, "whether or not to filter out endpoints not passing health checks")
	flag.StringVar(&Options.Socket, "docker", DEFAULT_DOCKER_SOCKET, "the location of the docker socket")
	flag.StringVar(&Options.Provider, "provider", "docker", "the services provider to use, either docker or static")
	flag.StringVar(&Options.Services, "services", "", "a comma seperated list of services i.e frontend;80,mysql;3306 etc")
	flag.IntVar(&Options.Forced_resync, "resync", DEFAULT_FORCED_RESYNC, "the amount of time in seconds to perform a resync of endpoints per service")
}
