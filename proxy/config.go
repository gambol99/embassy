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

package proxy

import (
	"fmt"
)

/*
Proxy Configuration
*/
type Configuration struct {
	/* the port the proxy is listening on */
	ProxyPort int
	/* the discovery uri i.e. etcd://localhost:4001 or consul://localhost:5000 etc */
	DiscoveryURI string
	/* the interface we bind to */
	Interface string
}

func (r Configuration) String() string {
	return fmt.Sprintf("configuration: interface: %s, port: %d, discovery: %s",
		r.Interface, r.ProxyPort, r.DiscoveryURI )
}
