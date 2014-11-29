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

package services

import (
	"fmt"
)

const (
	SERVICE_REQUEST	= 1 << iota
	SERVICE_CLOSED
)

type ServiceOperation int

func (s ServiceOperation) String() string {
	switch *s {
	case SERVICE_REQUEST:
		return "SERVICE_REQUEST"
	case SERVICE_CLOSED:
		return "SERVICE_CLOSED"
	default:
		return "UNKNOWN"
	}
}

type ServiceEvent struct {
	/* the operation type - adding or removing a service */
	Operation ServiceOperation
	/* the service definition */
	Service Service
}

func (s ServiceEvent) String() string {
	return fmt.Sprintf("action: %s, service: %s", s.Operation, s.Service )
}
