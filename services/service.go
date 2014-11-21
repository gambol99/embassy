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
	"strings"
)

type ServiceID string
type ServiceTags []string
type ServiceProtocol int

const (
	TCP ServiceProtocol = 1 << iota
	UDP
)

type Service struct {
	ID       ServiceID
	Name     string
	Protocol ServiceProtocol
	Tags     ServiceTags
	Port     int
}

func (s Service) String() string {
	return fmt.Sprintf("name: %s[%s]:%s/%s", s.Name, strings.Join(s.Tags, "|"), s.Port, s.ProtocolName())
}

func (s Service) ProtocolName() string {
	if s.isTCP() {
		return "tcp"
	}
	return "udp"
}

func (s Service) isTCP() bool {
	if s.Protocol == TCP {
		return true
	}
	return false
}

func (s Service) isUDP() bool {
	if !s.isTCP() {
		return true
	}
	return false
}
