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
	"fmt"
	"net"
	"sync"

	"github.com/gambol99/embassy/config"
	"github.com/gambol99/embassy/services"
	"github.com/gambol99/embassy/utils"
	"github.com/golang/glog"
)

type ProxyService interface {
	ProxyServices() error
}

type ProxyStore struct {
	/* the tcp listener */
	Listener net.Listener
	/* a map of the proxy [source_ip+service_port] to the proxier handler */
	Proxies map[ProxyID]Proxier
	/* channel for new service requests from the store */
	ServicesChannel services.ServiceStoreChannel
	/* channel for shutdown down */
	Shutdown utils.ShutdownSignalChannel
	/* service configuration */
	Config *config.Configuration
}

type ServiceMap struct {
	/* controls concurrent access to proxy services map */
	sync.RWMutex
	/* a map from proxy id to proxy service, we can have multiple id's pointing
	to the same proxy service
	 */
	ProxyServices map[ProxyID]ProxyService
}

func NewProxyStore(cfg *config.Configuration, store services.ServiceStore) (ProxyService, error) {
	glog.Infof("Creating a new proxy store")
	proxy := new(ProxyStore)
	proxy.Config = cfg

	/* step: create a channel to listen for new services from the store */
	glog.V(4).Infof("Creating a services channel for the proxy")
	proxy.ServicesChannel = make(services.ServiceStoreChannel)
	store.AddServiceListener(proxy.ServicesChannel)

	/* step: create a tcp listener for the proxy service */
	glog.V(2).Infof("Binding proxy to interface: %s:%d", cfg.IPAddress, cfg.ProxyPort)
	//tcp_addr := net.TCPAddr{net.ParseIP(cfg.IPAddress), cfg.ProxyPort, ""}
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.ProxyPort))
	if err != nil {
		glog.Errorf("Unable to bind proxy service, error: %s", err)
		return nil, err
	}
	proxy.Listener = listener

	/* step: create the map for holder proxiers */
	proxy.Proxies = make(map[ProxyID]ServiceProxy, 0)
	/* step: create the shutdown channel */
	proxy.ShutdownSignal = make(chan bool)

	/* step: start finding services */
	if err := store.FindServices(); err != nil {
		glog.Errorf("Failed to start the services stream, error: %s", err)
	}
	return proxy, nil
}

func (px *ProxyStore) ProxyServices() error {
	glog.Infof("Starting the TCP Proxy Service")
	/* step: lets start handling connections */
	if err := px.ProxyConnections(); err != nil {
		glog.Errorf("Failed to start listening for connections, error: %s", err)
		return err
	}
	for {
		select {
		case in := <-px.ShutdownSignal:
			glog.Infof("Received shutdown signal. Propagating the signal to the proxies: %d", len(px.Proxies))
			for _, proxier := range px.Proxies {
				glog.Infof("Sending the kill signal to proxy: %s", proxier)
			}
			var _ = in
		case service := <-px.ServicesChannel:
			glog.Infof("Recieved a new service request, adding the collection; service: %s", service)
			/* step: check if the service is already being processed */
			proxyID := GetProxyIDByService(&service)
			if _, found := px.LookupProxierByProxyID(proxyID); found {
				glog.Infof("Proxy for service: %s already found, skipping", service)
				continue
			}
			/* step: else we create a proxier for this service. We run this in a gorountine as we don't want delays
			in the startup code of a proxy to stop us from handling connection */
			go func() {
				glog.Infof("Creating new proxier for service: %s, consumer", service, proxyID)
				proxier, err := NewProxier(px.Config, service)
				if err != nil {
					glog.Errorf("Unable to create proxier, service: %s, error: %s", service, err)
					return
				}
				/* step; add the new proxier to the collection */
				px.AddServiceProxier(proxier)
			}()
		}
	}
	return nil
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

func (px *ProxyStore) LookupProxierByProxyID(id ProxyID) (proxier ServiceProxy, found bool) {
	px.RLock()
	defer px.RUnlock()
	proxier, found = px.Proxies[id]
	return
}

func (px *ProxyStore) AddServiceProxier(proxier ServiceProxy) {
	px.Lock()
	defer px.Unlock()
	px.Proxies[proxier.ID()] = proxier
	glog.V(3).Infof("Added proxyId: %s to collection of service proxies, size: %d", proxier.ID(), len(px.Proxies))
}

