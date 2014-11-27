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
	"strconv"
	"strings"

	"github.com/golang/glog"
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
	BD_DEFINITION    = regexp.MustCompile(`[[:alnum:]\/\*\.-]*(\[[[:alnum:],]+\])?;[0-9]+\/(tcp|udp)(;((.*)=(.*)){1,})?;?`)
	BD_SERVICE_NAME  = regexp.MustCompile(`(^[[:alnum:]\/\*\.-]*)`)
	BD_SERVICE_PORT  = regexp.MustCompile(`([0-9]+)`)
	BD_SERVICE_PROTO = regexp.MustCompile(`\/(tcp|udp)`)
	BD_SERVICE_TAGS  = regexp.MustCompile(`\[(.*)\]`)
)

func (b Definition) IsValid() bool {
	glog.V(6).Infof("Validating the service definition: %s", b.Definition)
	return BD_DEFINITION.MatchString(b.Definition)
}

func (b Definition) GetSection() func(int) string {
	var sections []string = strings.Split(b.Definition, ";")
	return func(index int) string {
		return sections[index]
	}
}

func (b Definition) String() string {
	return fmt.Sprintf("definition: %s|%s : %s ", b.SourceAddress, b.Name, b.Definition)
}

func (b Definition) GetService() (service Service, err error) {
	if matched := b.IsValid(); matched {
		var Section func(int) string = b.GetSection()
		service_name := Section(0)
		service.SourceIP = b.SourceAddress
		service.ID = ServiceID(b.Definition)
		service.Name = BD_SERVICE_NAME.FindStringSubmatch(service_name)[0]
		/* step: get the port */
		if converted, err := strconv.ParseInt(BD_SERVICE_PORT.FindStringSubmatch(Section(1))[0], 10, 64); err != nil {
			errors.New("Invalid service port")
		} else {
			if converted <= 0 || converted >= 65535 {
				errors.New("Invalid service port, must be between 1 and 65535")
			}
			service.Port = int(converted)
		}
		if strings.Index(service_name, "[") > 0 {
			service.Tags = strings.Split(BD_SERVICE_TAGS.FindStringSubmatch(service_name)[0], ",")
		}
		return service, nil
	} else {
		return service, errors.New("Invalid service definition, does not match requirements")
	}
}
