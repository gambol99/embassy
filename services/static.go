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
	"errors"

	"github.com/gambol99/embassy/config"
	"github.com/golang/glog"
)

type FixebBackendStore struct {
	Service Service
	Channel ServiceStoreChannel
}

func NewFixedServiceStore(config *config.Configuration, channel ServiceStoreChannel) (ServiceStore, error) {
	/* step: we create a docker client */
	glog.V(3).Infof("Creating Fixed Backend Store, backend: %s", config.FixedBackend)
	/* step: valid the definition */
	var definition BackendDefiniton
	definition.Name = "Fixed Backend"
	definition.Definition = config.FixedBackend
	service, err := definition.GetService()
	if err != nil {
		glog.Errorf("The fixed backend is invalid, error: %s")
		return nil, errors.New("The fixed backend definition is invalid, please check")
	}
	return &FixebBackendStore{service, channel}, nil
}

func (fx *FixebBackendStore) DiscoverServices() error {
	glog.V(5).Infof("Pushing the fixed backend into channel")
	fx.Channel <- fx.Service
	return nil
}
