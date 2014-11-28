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
	"github.com/gambol99/embassy/services"
)

type EndpointsCache struct {
	stores map[ServiceID]EndpointsStore
}

func (r *EndpointsCache) Lookup(si *services.Service) (store EndpointsStore, found bool) {
	if store, found = r.store[si.ID]; found {
		return
	}
	return nil, false
}

func (r *EndpointsCache) Add(si *services.Service, store EndpointsStore) {
	store[si.ID] = store
}

func (r *EndpointsCache) Remove(si *services.Service) {
	r.stores = delete[si.ID]
}
