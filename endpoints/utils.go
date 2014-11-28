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

package discovery

import (
	"errors"
	"regexp"

	"github.com/gambol99/embassy/config"
	"github.com/gambol99/embassy/services"
	"github.com/golang/glog"
)

const (
	DISCOVERY_STORES = `^(consul|etcd):\/\/`
)

func NewDiscoveryService(cfg *config.Configuration, si services.Service) (DiscoveryStore, error) {
	/* step: check the cache first of all */
	glog.Infof("Creating a new discovery agent for service: %s", si)
	/* step: check if the store provider is supported */
	if !IsDiscoveryStore(cfg.DiscoveryURI) {
		return nil, errors.New("The backend discovery store specified is not supported")
	}

	var provider DiscoveryStoreProvider
	var err error
	discovery := new(DiscoveryStoreService)
	discovery.Service = si
	discovery.Config = cfg
	discovery.Endpoints = make([]services.Endpoint, 0)

	/* step: create the backend provioder */
	switch cfg.GetDiscoveryURI().Scheme {
	case "etcd":
		glog.Infof("Using Etcd as discovery backend, uri: %s", cfg.DiscoveryURI)
		provider, err = NewEtcdStore(cfg)
	default:
		glog.Errorf("The discovery backend %s is not supported, please check usage", cfg.DiscoveryURI)
		return nil, errors.New("Invalid discovery backend")
	}
	if err != nil {
		glog.Errorf("Unable to initialize the Etcd backend store, error: %s", err)
		return nil, err
	}
	discovery.Store = provider
	return discovery, nil
}

func IsDiscoveryStore(uri string) bool {
	var validator = regexp.MustCompile(DISCOVERY_STORES)
	if found := validator.MatchString(uri); found {
		return true
	}
	glog.Errorf("The backend: %s is not supported, please check usage", uri)
	return false
}
