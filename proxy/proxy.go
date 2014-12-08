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
	"fmt"
	"syscall"
	"strconv"

	"github.com/gambol99/embassy/proxy/services"
	"github.com/gambol99/embassy/utils"
	"github.com/golang/glog"
)

type ProxyService interface {
	Close()
	Start() error
}

type ProxyStore struct {
	sync.RWMutex
	/* the tcp listener */
	Listener net.Listener
	/* a map of the proxy [source_ip+service_port] to the proxy service handler */
	Proxies map[ProxyID]ServiceProxy
	/* channel for shutdown down */
	Shutdown utils.ShutdownSignalChannel
	/* service configuration */
	Config Configuration
	/* the services store */
	Store services.ServiceStore
}

/* --------------- Proxy ID ------------------- */

type ProxyID string

func (p *ProxyID) String() string {
	return fmt.Sprintf("proxyId: %s", *p )
}

func GetProxyIDByConnection(source, port string) ProxyID {
	return ProxyID(fmt.Sprintf("%s:%s", source, port))
}

func GetProxyIDByService(si *services.Service) ProxyID {
	return ProxyID(fmt.Sprintf("%s:%d", si.Consumer, si.Port))
}

/* --------------- Proxy Service -------------- */

func (px *ProxyStore) Close() {
	glog.V(4).Infof("Request to shutdown the proxy services")
	px.Shutdown <- true
}

func (px *ProxyStore) Start() error {
	glog.Infof("Starting the TCP Proxy Service")

	/* step: add ourselves us a listener to service events */
	glog.V(5).Infof("Initializing the service discovery and starting to listen")
	service_updates := make(services.ServiceEventsChannel, 10 )
	defer close(service_updates)
	px.Store.AddServiceListener(service_updates)
	/* step: we need to start listening for services */
	if err := px.Store.Start(); err != nil {
		glog.Errorf("Failed to start the service discovery subsystem, error: %s", err )
		return err
	}

	/* step: start the connection handling */
	if err := px.ProxyConnections(); err != nil {
		glog.Errorf("Failed to start listening for connections, error: %s", err)
		return err
	}

	/* step: the main event loop for the proxy service;
		- listening for service events
		- listening for shutdown requests
	 */
	for {
		select {
		case <-px.Shutdown:
			glog.Infof("Received shutdown signal. Propagating the signal to the proxies: %d", len(px.Proxies))
			for _, proxier := range px.Proxies {
				glog.Infof("Sending the kill signal to proxy: %s", proxier )
				proxier.Close()
			}
		case event := <-service_updates:
			glog.Infof("ProxyService recieved service update, service: %s, action: %s", event.Service, event.Action )
			/* step: check if the service is already being processed */
			if err := px.ProcessServiceEvent(&event); err != nil {
				glog.Errorf("Unable to process the service request: %s, error: %s", event.Service, err )
			}
		}
	}
	return nil
}

func (px *ProxyStore) ProcessServiceEvent(event *services.ServiceEvent) error {
	/* check: is this a service request or down */
	if event.IsServiceRequest() {
		go px.CreateServiceProxy(event.Service)
	} else if event.IsServiceRemoval() {
		go px.DestroyServiceProxy(event.Service)
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
	glog.V(5).Infof("Starting to listen for incoming connections" )
	go func() {
		for {
			/* wait for a connection */
			conn, err := px.Listener.Accept()
			if err != nil {
				glog.Errorf("Failed to accept connection, error: %s", err)
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
				original_port, err := px.GetOriginalPort(connection)
				if err != nil {
					glog.Errorf("Unable to get the original port, connection: %s, error: %s", connection.RemoteAddr(), err)
					connection.Close()
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
			}(conn.(*net.TCPConn))
		}
	}()
	return nil
}

/*
Derive the original port from the tcp header
 */
func (px *ProxyStore) GetOriginalPort(conn *net.TCPConn) (string, error) {
	descriptor, err := conn.File()
	if err != nil {
		glog.Errorf("Unable to get tcp descriptor, connection: %s, error: ", conn.RemoteAddr(), err)
		return "", err
	}
	addr, err := syscall.GetsockoptIPv6Mreq(int(descriptor.Fd()), syscall.IPPROTO_IP, SO_ORIGINAL_DST)
	if err != nil {
		glog.Errorf("Unable to get the original destination port for connection: %s, error: %s", conn.RemoteAddr(), err)
		return "", err
	}
	destination := uint16(addr.Multiaddr[2])<<8 + uint16(addr.Multiaddr[3])
	return strconv.Itoa(int(destination)), nil
}

func (px *ProxyStore) LookupProxierByServiceId(id services.ServiceID) (ServiceProxy,bool) {
	for _, service := range px.Proxies {
		if service.GetService().ID == id {
			glog.V(3).Infof("Found service proxy for service id: %s", id )
			return service, true
		}
	}
	glog.V(3).Infof("Proxy service does not exists for service id: %s", id )
	return nil, false
}

func (px *ProxyStore) LookupProxierByProxyID(id ProxyID) (proxy ServiceProxy, found bool) {
	px.RLock()
	defer px.RUnlock()
	proxy, found = px.Proxies[id]
	return
}

func (px *ProxyStore) CreateServiceProxy(si services.Service) error {
	/* step: get the proxy id for this service */
	proxyID := GetProxyIDByService(&si)
	/* step: check if a service proxy already exists */
	px.Lock()
	defer px.Unlock()
	/*
	if proxy, found := px.LookupProxierByServiceId(si.ID); found {
		// the proxy service already exists, we simply map a proxyID -> ProxyService
		glog.V(3).Infof("A proxy service already exists for service: %s, mapping new proxy id: %s", si, proxyID )
		px.AddServiceProxy(proxyID, proxy )
	} else {
	*/
		glog.Infof("Creating new proxy service for service: %s, consumer", si, proxyID )
		proxy, err := NewServiceProxy(si, px.Config.DiscoveryURI )
		if err != nil {
			glog.Errorf("Unable to create proxier, service: %s, error: %s", si, err )
			return err
		}
		/* step; add the new proxier to the collection */
		px.AddServiceProxy(proxyID, proxy)
	//}
	return nil
}

func (px *ProxyStore) DestroyServiceProxy(si services.Service) error {
	/* step: get the proxy id for this service */
	proxyID := GetProxyIDByService(&si)
	px.Lock()
	defer px.Unlock()
	count := 0
	for _, proxy := range px.Proxies {
		if proxy.GetService().ID == si.ID {
			count++
		}
	}
	/* step: does we have multiple mappings */
	if count == 0 {
		glog.Errorf("Unable to find a service proxy for service: %s", si )
		return errors.New("Service Proxy does not exists")
	}
	if count == 1 {
		glog.Infof("Releasing the resources of service proxy, service: %s", si )
		proxy, _ := px.Proxies[proxyID]
		proxy.Close()
	} else {
		glog.Infof("Service proxy for service: %s has multiple mappings", si)
	}
	delete(px.Proxies, proxyID )
	return nil
}

func (px *ProxyStore) AddServiceProxy(proxyID ProxyID, proxy ServiceProxy) {
	glog.V(8).Infof("Adding Service Proxy, service: %s, service id: %s", proxy.GetService(), proxy.GetService().ID )
	px.Proxies[proxyID] = proxy
	glog.V(3).Infof("Added proxyId: %s to collection of service proxies, size: %d", proxyID, len(px.Proxies))
}
