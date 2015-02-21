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

package store

import (
	"errors"
	"strings"

	"github.com/gambol99/embassy/config"
	"github.com/gambol99/embassy/proxy/services"
	"github.com/gambol99/embassy/utils"

	"github.com/golang/glog"
)

type StaticServiceStore struct{}

func NewStaticServiceStore() (services.ServiceStore, error) {
	// step: check we have been passed the services field
	if config.Options.Services == "" {
		return nil, errors.New("You have specified any services to proxy, check usage menu")
	}
	return &StaticServiceStore{}, nil
}

func (r *StaticServiceStore) Close() error {
	return nil
}

func (r *StaticServiceStore) StreamServices(channel services.ServicesChannel) error {
	// step: we need to get the ip address
	address, err := utils.GetLocalIPAddress(config.Options.Proxy_interface)
	if err != nil {
		return err
	}

	// step: extract the labels from the services
	definitions := make([]Definition, 0)
	for _, label := range strings.Split(config.Options.Services, ",") {
		definition := Definition{
			Name:          label,
			SourceAddress: address,
			Definition:    label,
		}
		if definition.IsValid() {
			definitions = append(definitions, definition)
		} else {
			glog.Errorf("The service definition: %s is invalid, please review", label)
		}
	}
	pushServices(channel, definitions, true)

	return nil
}
