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
	SERVICE_REMOVAL
)

type Operation int

type ServiceEvent struct {
	/* the operation type - adding or removing a service */
	Action Operation
	/* the service definition */
	Service Service
}

func (r ServiceEvent) String() string {
	return fmt.Sprintf("action: %s, service: %s", r.Action, r.Service )
}

func (r ServiceEvent) IsServiceRequest() bool {
	if r.Action == SERVICE_REQUEST {
		return true
	}
	return false
}

func (r ServiceEvent) IsServiceRemoval() bool {
	if r.Action == SERVICE_REMOVAL {
		return true
	}
	return false
}
