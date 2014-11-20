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

package utils

import (
	"errors"
	"net"
	"strings"

	"github.com/golang/glog"
)

const (
	PROXY_INTERFACE = "eth0"
)

func GetLocalIPAddress(interface_name string) (string, error) {
	glog.V(5).Infof("Attempting to grab the ipaddress of interface: %s", interface_name)
	if interface_name == "" {
		interface_name = PROXY_INTERFACE
	}
	if interfaces, err := net.Interfaces(); err != nil {
		glog.Errorf("Unable to get the proxy ip address, error: %s", err)
		return "", err
	} else {
		for _, iface := range interfaces {
			glog.V(5).Infof("Pulling the ip addresses, interface: %s", iface.Name)
			/* step: get only the interface we're interested in */
			if iface.Name == interface_name {
				glog.V(6).Infof("Found interface: %s, grabbing the ip addresses", iface.Name)
				addrs, err := iface.Addrs()
				if err != nil {
					glog.Errorf("Unable to retrieve the ip addresses on interface: %s, error: %s", iface.Name, err)
					return "", err
				}
				/* step: return the first address */
				if len(addrs) > 0 {
					return strings.SplitN(addrs[0].String(), "/", 2)[0], nil
				} else {
					glog.Fatalf("The interface: %s has no ip address", interface_name)
				}
			}
		}
	}
	return "", errors.New("Unable to determine or find the interface")
}
