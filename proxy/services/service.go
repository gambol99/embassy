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

	"github.com/gambol99/embassy/utils"
)

type ServiceID string

type Service struct {
	ID       ServiceID
	Name     string
	Consumer string
	Port     int
}

func (s Service) IsValid() error {
	if s.Name == "" {
		return errors.New("Service does not have a name field")
	}
	if _, err := utils.IsPort(s.Port); err != nil {
		return errors.New("Invalid service port, check the service document")
	}
	return nil
}

func (s Service) String() string {
	return fmt.Sprintf("name: %s:%d, ip: %s", s.Name, s.Port, s.Consumer)
}
