/*
Copyright 2014 Rohith Jayawaredene All rights reserved.

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

package main

import (
	"errors"
	"net"

	"github.com/golang/glog"
)

const (
	PROXY_INTERFACE = "eth0"
)

func GetLocalIPAddress(interface_name string) (string, error) {
	if interface_name == "" {
		interface_name = PROXY_INTERFACE
	}
	if interfaces, err := net.Interfaces(); err != nil {
		glog.Errorf("Unable to get the proxy ip address, error: %s", err)
		return "", err
	} else {
		for _, iface := range interfaces {
			/* step: get only the interface we're interested in */
			if iface.Name == interface_name {
				if addrs, err := iface.Addrs(); err != nil {
					glog.Errorf("Unable to retrieve the ip addresses on interface: %s, error: %s", iface.Name, err)
					return "", err
				} else {
					for _, address := range addrs {
						switch address.(type) {
						case *net.IPAddr:
							return address.String(), nil
						}
					}
				}
			}
		}
	}
	return "", errors.New("Unable to determine or find the interface")
}
