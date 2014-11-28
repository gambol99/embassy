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

package discovery

import (
	"fmt"
)

type EndPoint string
type EndPointEvent int

const (
	CHANGED = 1 << iota
	DELETED
)

type EndpointEvent struct {
	Name     string
	Event    EndPointEvent
}

func (r EndpointEvent) String() string {
	return fmt.Sprintf("event: [%s] %s", r.Name, r.Event)
}
