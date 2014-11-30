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

package proxy

import (
	"net"
	"sync"
	"errors"

	"github.com/gambol99/embassy/config"
	"github.com/gambol99/embassy/services"
	"github.com/gambol99/embassy/utils"
	"github.com/golang/glog"
)

type ProxyService interface {
	Close() error
	Start() error
}

type ProxyStore struct {
	sync.RWMutex
	/* the tcp listener */
	Listener net.Listener
	/* a map of the proxy [source_ip+service_port] to the proxy service handler */
	Proxies map[ProxyID]ProxyService
	/* a map of the services to proxies */
	ServiceMap map[services.ServiceID]ProxyService
	/* channel for new service requests from the store */
	ServicesChannel services.ServiceStoreChannel
	/* channel for shutdown down */
	Shutdown utils.ShutdownSignalChannel
	/* service configuration */
	Config *config.Configuration
}

func (px *ProxyStore) Close() {
	glog.V(4).Infof("Request to shutdown the proxy services")
	px.Shutdown <- true
}

func (px *ProxyStore) Start() error {
	glog.Infof("Starting the TCP Proxy Service")
	/* step: lets start handling connections */
	if err := px.ProxyConnections(); err != nil {
		glog.Errorf("Failed to start listening for connections, error: %s", err)
		return err
	}
	for {
		select {
		case <-px.Shutdown:
			glog.Infof("Received shutdown signal. Propagating the signal to the proxies: %d", len(px.Proxies))
			for _, proxier := range px.Proxies {
				glog.Infof("Sending the kill signal to proxy: %s", proxier)
			}
		case service := <-px.ServicesChannel:
			glog.Infof("Recieved a new service request, adding the collection; service: %s", service)
			/* step: check if the service is already being processed */
			if err := px.ProcessServiceEvent(service); err != nil {
				glog.Errorf("Unable to process the service request, error: %s", err )
			}
		}
	}
	return nil
}

func (px *ProxyStore) ProcessServiceEvent(si *services.ServiceEvent) error {
	/* check: is this a service request or down */
	switch si.Operation {
	case services.SERVICE_REQUEST:
		proxy, found := px.LookupProxyByServiceId(si.Service.ID)
		if found {
			proxyID := GetProxyIDByService(&si.Service)
			/* the proxy service already exists, we simply map a proxyID -> ProxyService */
			glog.V(3).Infof("A proxy service already exists for service: %s, mapping new proxy id: %s", si.Service, proxyID )
			px.AddServiceProxy(proxyID, proxy )
		} else {
			/* step: else we create a proxy service for this request. We run this in a go rountine as we don't want delays
			in the startup code of a proxy to stop us from handling connection */
			go func() {
				service := si.Service
				proxyID := GetProxyIDByService(&service)
				glog.Infof("Creating new proxy service for service: %s, consumer", service, proxyID )
				proxier, err := NewProxier(px.Config, service)
				if err != nil {
					glog.Errorf("Unable to create proxier, service: %s, error: %s", service, err )
					return
				}
				/* step; add the new proxier to the collection */
				px.AddServiceProxy(proxyID,proxier)
			}()
		}
	case services.SERVICE_CLOSED:
	default:
		glog.Errorf("Unknown service request operation, service: %s", si )
		return errors.New("Unknow service request operation")
	}
	return nil
}

func (px *ProxyStore) LookupProxyByServiceId(id services.ServiceID) (service ProxyService, found bool) {
	service, found := px.ServiceMap[id]
	return
}

/*
	- Wait for connections on the tcp listener
	- Get the original port before redirection
	- Lookup the service proxy for this service (src_ip + original_port)
	- Pass the connection to the in a go handler
*/
func (px *ProxyStore) ProxyConnections() error {
	go func() {
		for {
			/* wait for a connection */
			conn, err := px.Listener.Accept()
			if err != nil {
				glog.Errorf("Accept connection failed: %s", err)
				continue
			}
			/* handle the rest with in go routine and return to pick up another connection */
			go func(connection *net.TCPConn) {
				/* step: get the destination port and source ip address */
				source_ipaddress, _, err := net.SplitHostPort(connection.RemoteAddr().String())
				if err != nil {
					glog.Errorf("Unable to get the remote ipaddress and port, error: %s", err)
					connection.Close()
				}
				/* step: get the original port */
				original_port, err := GetOriginalPort(connection.(*net.TCPConn))
				if err != nil {
					glog.Errorf("Unable to get the original port, connection: %s, error: %s", connection.RemoteAddr(), err)
					connection.Close()
					continue
				}

				glog.V(5).Infof("Accepted TCP connection from %v to %v, original port: %s",
					connection.RemoteAddr(), connection.LocalAddr(), original_port)
				/* step: create a proxyId for this */
				proxyId := GetProxyIDByConnection(source_ipaddress, original_port)
				/* step: find the service proxy responsible for handling this service */
				if proxier, found := px.LookupProxierByProxyID(proxyId); found {
					/* step: handle the connection in the proxier */
					go func() {
						if err := proxier.HandleTCPConnection(connection); err != nil {
							glog.Errorf("Failed to handle the connection: %s, proxyid: %s, error: %s", connection.RemoteAddr(),
								proxyId, err)
							if err := connection.Close(); err != nil {
								glog.Errorf("Failed to close the connection connection: %s, error: %s", connection.RemoteAddr(), err)
							}
						}
					}()
				} else {
					glog.Errorf("Failed to handle service, we do not have a proxier for: %s", proxyId)
					connection.Close()
				}
			}(conn)
		}
	}()
	return nil
}

func (px *ProxyStore) LookupProxierByProxyID(id ProxyID) (proxy ServiceProxy, found bool) {
	px.RLock()
	defer px.RUnlock()
	proxy, found = px.Proxies[id]
	return
}

func (px *ProxyStore) AddServiceProxy(proxyID ProxyID, proxy ServiceProxy) {
	px.Lock()
	defer px.Unlock()
	px.Proxies[proxyID] = proxy
	glog.V(3).Infof("Added proxyId: %s to collection of service proxies, size: %d", proxyID, len(px.Proxies))
}

