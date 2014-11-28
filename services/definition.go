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
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/gambol99/embassy/utils"
)

/*
  SERVICE=<SERVICE_NAME>[TAGS,..];<PORT>;[PROTOCOL];
  BACKEND=etcd://localhost:4001
  BACKEND_REDIS_MASTER=redis.master;PORT
  BACKEND_REDIS_MASTER=redis.master[prod,dc1]
  BACKEND_REDIS_MASTER=/services/prod/redis/master/6379/*;PORT;OPTION=VALUE,;
*/

type Definition struct {
	SourceAddress, Name, Definition string
}

var (
	BD_DEFINITION   = regexp.MustCompile(`([[:alnum:]\/\.\-\_*]+)(\[(.*)\])?;([[:digit:]]{1,5})\/(tcp|udp)`)
	BD_SERVICE_NAME = regexp.MustCompile(`([[:alnum:]\/\.\-\_*]+)`)
	BD_SERVICE_PORT = regexp.MustCompile(`([[:digit:]]+)\/(tcp|udp)`)
	BD_SERVICE_TAGS = regexp.MustCompile(`\[(.*)\]`)
)

func (b Definition) IsValid() bool {
	return BD_DEFINITION.MatchString(b.Definition)
}

func (b Definition) String() string {
	return fmt.Sprintf("definition: %s|%s : %s ", b.SourceAddress, b.Name, b.Definition)
}

/* /services/prod/redis/master/6379/*;PORT;OPTION=VALUE */
func (b Definition) GetService() (service Service, err error) {
	if matched := b.IsValid(); matched {
		sections := strings.Split(b.Definition, ";")
		section_name := sections[0]
		section_network := sections[1]

		service.ID = ServiceID(b.Definition)
		service.SourceIP = b.SourceAddress
		service.Name = BD_SERVICE_NAME.FindStringSubmatch(section_name)[0]
		service.Port, err = utils.ToInteger(BD_SERVICE_PORT.FindAllStringSubmatch(section_network, 1)[0][1])
		if err != nil {
			return service, errors.New("Invalid service port found in defintion")
		}
		protocol := BD_SERVICE_PORT.FindAllStringSubmatch(section_network, 1)[0][2]
		switch protocol {
		case "tcp":
			service.Proto = TCP
		case "udp":
			service.Proto = UDP
		}
		/* step: get the service tags */
		if strings.Index(section_name, "[") > 0 {
			service.Tags = strings.Split(BD_SERVICE_TAGS.FindStringSubmatch(section_name)[0], ",")
		}
	} else {
		return service, errors.New("Invalid service definition, does not match requirements")
	}
	return
}
