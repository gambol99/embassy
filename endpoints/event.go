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

	"github.com/gambol99/embassy/services"
)

type EndPointChangeAction int

const (
	CHANGED = 1 << iota
	DELETED
)

type EndpointChangedEvent struct {
	Path 	 string
	Event 	 EndPointChangeAction
}

func (r EndpointChangedEvent) String() string {
	return fmt.Sprintf("event: [%s] %s", r.Path, r.Event )
}

/*
The event which is sent to the listeners
 */
type EndpointEvent struct {
	Event    EndPointChangeAction
	Service  services.Service
}

func (r EndpointEvent) String() string {
	return fmt.Sprintf("action: %s, service: %s", r.Event, r.Service )
}

