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

package endpoints

import (
	"fmt"
	"time"

	"github.com/gambol99/embassy/proxy/services"
)

type Operation int

const (
	ENDPOINT_CHANGED = 1 << iota
	ENDPOINT_REMOVED
)

/*
The event which is sent to the listeners
 */
type EndpointEvent struct {
	ID       string
	Stamp    time.Time
	Action   Operation
	Service  services.Service
}

func (r EndpointEvent) String() string {
	return fmt.Sprintf("time: %s, id: %s, action: %s, service: %s", r.Stamp, r.ID, r.Action, r.Service )
}

func (r EndpointEvent) IsChanged() bool {
	if r.Action == ENDPOINT_CHANGED {
		return true
	}
	return false
}

func (r EndpointEvent) IsRemoved() bool {
	if r.Action == ENDPOINT_REMOVED {
		return true
	}
	return false
}
