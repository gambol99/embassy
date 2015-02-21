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
	"fmt"
	"regexp"
	"strings"

	"github.com/gambol99/embassy/proxy/services"
	"github.com/gambol99/embassy/utils"
)

type Definition struct {
	// The source ip of the container request the service
	SourceAddress string
	// The name of the service extracted from the definition
	Name string
	// The service definition itself
	Definition string
	// The operation type, i.e. adding or removing
	Adding bool
}

func (b Definition) String() string {
	return fmt.Sprintf("definition: %s|%s : %s operation: %t ", b.SourceAddress, b.Name, b.Definition, b.Adding)
}

var (
	BD_DEFINITION   = regexp.MustCompile(`([[:alnum:]\/\.\-\_*]+);([[:digit:]]{1,5})`)
	BD_SERVICE_NAME = regexp.MustCompile(`([[:alnum:]\/\.\-\_*]+)`)
	BD_SERVICE_PORT = regexp.MustCompile(`([[:digit:]]+)`)
)

// Check the definition against the validation regex
func (b Definition) IsValid() bool {
	return BD_DEFINITION.MatchString(b.Definition)
}

// Converting a service definition into a Service or error
func (b Definition) GetService() (service services.Service, err error) {
	if matched := b.IsValid(); matched {
		sections := strings.Split(b.Definition, ";")
		section_name := sections[0]
		section_network := sections[1]

		service.ID = services.ServiceID(b.Definition)
		service.Consumer = b.SourceAddress
		service.Name = BD_SERVICE_NAME.FindStringSubmatch(section_name)[0]
		service.Port, err = utils.ToInteger(BD_SERVICE_PORT.FindAllStringSubmatch(section_network, 1)[0][1])
		if err != nil {
			return service, errors.New("Invalid service port found in defintion: " + b.Definition)
		}
	} else {
		return service, errors.New("Invalid service definition, does not match requirements: " + b.Definition)
	}
	return
}
