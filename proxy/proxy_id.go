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

	"github.com/gambol99/embassy/services"
)

type ProxyID string

func (p *ProxyID) String() string {
	return fmt.Sprintf("proxyId: %s", p )
}

func GetProxyIDByConnection(source, port string) ProxyID {
	return ProxyID(fmt.Sprintf("%s:%s", source, port))
}

func GetProxyIDByService(si *services.Service) ProxyID {
	return ProxyID(fmt.Sprintf("%s:%d", si.SourceIP, si.Port))
}
