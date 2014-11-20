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

package services

import (
	"fmt"
	"net"
)

type EndpointID string

type Endpoint struct {
	ID      EndpointID
	Address net.Addr
	Port    int
}

func (s Endpoint) String() string {
	return fmt.Sprintf("id: %s, address: %s, port: %d", s.ID, s.Address, s.Port)
}
