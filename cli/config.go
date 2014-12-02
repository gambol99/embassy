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
package cli

import (
	"flag"

	"github.com/gambol99/embassy/proxy"
)

const (
	DEFAULT_PROXY_PORT     = 9999
	DEFAULT_INTERFACE      = "eth0"
	DEFAULT_DISCOVERY_URI  = "etcd://localhost:4001"
	DEFAULT_SERVICE_PREFIX = "BACKEND_"
)

var (
	cfg_discovery = flag.String("discovery", DEFAULT_DISCOVERY_URI, "the discovery backend to pull the services from")
	cfg_iface     = flag.String("interface", DEFAULT_INTERFACE, "the interface to take the proxy address from")
	cfg_proxy_port = flag.Int("port", DEFAULT_PROXY_PORT, "the tcp port which the proxy should listen on")
)

/*
Process the command line options and produce a proxy.Configuration
 */
func ParseOptions() (config proxy.Configuration) {
	flag.Parse()
	config.Interface = *cfg_iface
	config.ProxyPort = *cfg_proxy_port
	config.DiscoveryURI = *cfg_discovery
	return
}
