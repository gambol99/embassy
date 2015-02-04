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
	DEFAULT_PROXY_PORT 		= 9999
	DEFAULT_INTERFACE  		= "eth0"
	DEFAULT_DISCOVERY_URI  	= "consul://127.0.0.1:8500"
	DEFAULT_SERVICE_PREFIX 	= "BACKEND_"
)

var Options struct {
	/* the interface the proxy should be listening */
	Proxy_interface string
	/* the proxy port */
	Proxy_port int
	/* the discovery provider for service */
	Discovery_url string
	/* the service prefix */
	Service_prefix string
}

func init() {
	flag.StringVar(&Options.Proxy_interface, "interface", DEFAULT_INTERFACE, "the interface to take the proxy address from")
	flag.IntVar(&Options.Proxy_port, "port", DEFAULT_PROXY_PORT, "the tcp port which the proxy should listen on")
	flag.StringVar(&Options.Discovery_url, "discovery", DEFAULT_DISCOVERY_URI, "the discovery backend to pull the services from")
	flag.StringVar(&Options.Service_prefix, "prefix", DEFAULT_SERVICE_PREFIX, "the prefix used to distinguish a backend service")
}
