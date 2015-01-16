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
	"flag"
	"errors"
	"strings"

	"github.com/gambol99/embassy/utils"
	"github.com/golang/glog"
	"github.com/gambol99/embassy/proxy"
)

var (
	static_services *string
)

func init() {
	static_services = flag.String("services", "", "a comma seperated list of services i.e frontend;80,mysql;3306 etc")
}

func AddStaticServiceProvider() (ServiceProvider,error) {
	/* step: check we have been passed the services field */
	if *static_services == "" {
		return nil, errors.New("You have specified any services to proxy, check usage menu")
	}
	/* step: we need to get the ip address */
	address, err := utils.GetLocalIPAddress(*proxy.Proxy_interface)
	if err != nil {
		glog.Errorf("Failed to get the ip address of %s interface, error: %s", proxy.Proxy_interface, err )
		return nil, err
	}

	/* step: create the static services */
	return &StaticServiceStore{*static_services,address}, nil
}

type StaticServiceStore struct {
	/* the service we are proxying */
	services string
	/* the ip address of the container */
	address string
}

func (r *StaticServiceStore) StreamServices(channel BackendServiceChannel) error {
	glog.Infof("Extracting the services from: %s", r.services)

	/* step: extract the labels from the services */
	definitions := make([]DefinitionEvent, 0)
	for _, label := range strings.Split(r.services, ",") {
		var definition DefinitionEvent
		definition.Name = label
		definition.SourceAddress = r.address
		definition.Definition = label
		definition.Operation = DEFINITION_SERVICE_ADDED
		/* step: validate the service */
		if definition.IsValid() {
			definitions = append(definitions, definition)
		} else {
			glog.Errorf("The service definition: %s is invalid, please review", label )
		}
	}

	/* step: we forward the definitions along the channel */
	for _, definition := range definitions {
		glog.V(3).Infof("Pushing service: %s to services store", definition)
		channel <- definition
	}
	return nil
}
