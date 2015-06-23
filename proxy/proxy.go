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
	"strconv"
	"syscall"

	"github.com/gambol99/embassy/config"
	"github.com/gambol99/embassy/proxy/services"
	"github.com/gambol99/embassy/utils"

	"github.com/golang/glog"
)

const (
	REAL_PANIC      = false
	SO_ORIGINAL_DST = 80
)

type Proxy interface {
	/* shutdown the proxy service */
	Close()
	/* start handling the events */
	ProxyConnections() error
}

type ProxyStore struct {
	/* the tcp listener */
	listener net.Listener
	/* channel for shutdown down */
	shutdown utils.ShutdownSignalChannel
	/* the services store */
	store services.ServiceStore
}

type ProxyDefinition struct {
	// The proxyId to associated to the above
	ProxyID ProxyID

	// the service proxy
	Proxier ServiceProxy
}

type ProxyConnection struct {
	// The connection from the incoming request
	Connection net.Conn

	// The ProxyID associated to this request
	ProxyID ProxyID
}

/* Create the proxy service - the main routine for handling requests and events */
func NewProxyService(store services.ServiceStore) (Proxy, error) {
	glog.Infof("Initializing the ProxyService")

	/* step: we need to grab the ip address of the interface to bind to */
	ipaddress, err := utils.GetLocalIPAddress(config.Options.Proxy_interface)
	if err != nil {
		glog.Error("Unable to get the local ip address from interface: %s, error: %s", config.Options.Proxy_interface, err)
		return nil, err
	}

	/* step: bind to the interface */
	glog.Infof("Binding proxy service to interface: %s:%d", ipaddress, config.Options.Proxy_port)
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", config.Options.Proxy_port))
	if err != nil {
		glog.Errorf("Failed to bind the proxy service to interface, error: %s", err)
		return nil, err
	}

	/* step: build out the proxy service */
	service := new(ProxyStore)
	service.store = store
	service.shutdown = make(utils.ShutdownSignalChannel)
	service.listener = listener
	return service, nil
}

/* --------------- Proxy Service -------------- */

func (px *ProxyStore) Close() {
	glog.V(4).Infof("Request to shutdown the proxy services")
	px.shutdown <- true
}

func (px *ProxyStore) AcceptConnections(channel chan *ProxyConnection) {
	glog.V(5).Infof("Starting to listen for incoming connections")
	for {
		// We block and wait for a incoming connection
		if conn, err := px.listener.Accept(); err != nil {
			glog.Errorf("Failed to accept connection, error: %s", err)
		} else {
			// We need to extract the remote address i.e the client ip address
			ipaddress, _, err := net.SplitHostPort(conn.RemoteAddr().String())
			if err != nil {
				glog.Errorf("Unable to get the remote ipaddress and port, error: %s", err)
				conn.Close()
				continue
			}
			// We need to extract the original port from the connection
			port, err := px.GetOriginalPort(conn.(*net.TCPConn))
			if err != nil {
				glog.Errorf("Unable to get the original port, connection: %s, error: %s", conn.RemoteAddr(), err)
				conn.Close()
				continue
			}

			glog.V(5).Infof("Accepted TCP connection from %v to %v, original port: %s", conn.RemoteAddr(), conn.LocalAddr(), port)
			proxyID := GetProxyIDByConnection(ipaddress, port)
			channel <- &ProxyConnection{conn, proxyID}
		}
	}
}

/*
- Wait for connections on the tcp listener
- Get the original port before redirection
- Lookup the service proxy for this service (src_ip + original_port)
- Pass the connection to the in a go handler
*/
func (px *ProxyStore) ProxyConnections() error {

	// We receive updates from the service store on this channel
	create := make(chan ProxyDefinition, 20)
	destroy := make(chan ProxyDefinition, 20)
	incoming := make(chan *ProxyConnection, 500)
	bindings := make(services.ServicesChannel, 20)
	// Mappings for proxyID => proxy and serviceID => count
	proxyIDs := make(map[ProxyID]ServiceProxy, 0)
	serviceIDs := make(map[services.ServiceID]int, 0)

	glog.V(6).Infof("Binding to the services stores for service events")
	if err := px.store.StreamServices(bindings); err != nil {
		glog.Errorf("Failed to add our self as a service listener, error: %s", err)
		return err
	}

	// start listening for proxy connections
	go px.AcceptConnections(incoming)

	for {
		select {
		// We have an income proxy request event
		case request := <-incoming:
			if proxy, found := proxyIDs[request.ProxyID]; found {
				go proxy.HandleTCPConnection(request)
			}

		// We add a service proxy to the proxy map
		case request := <-create:
			proxyIDs[request.ProxyID] = request.Proxier
			serviceIDs[request.Proxier.GetService().ID] += 1

		// We remove a proxy from the service map
		case request := <-destroy:
			delete(proxyIDs, request.ProxyID)
			serviceID := request.Proxier.GetService().ID
			count, found := serviceIDs[serviceID]

			// Something VERY ODD - a mapping does not exist: we have a bug somewhere!!
			if !found {
				glog.Errorf("Something very ODD here, the proxy: %s has not been mapped", request.Proxier)
				continue
			} else if count == 1 {
				delete(serviceIDs, serviceID)
				go request.Proxier.Close()
				continue
			} else {
				// Default behaviour, we have multiple mappings, decrement and move on
				glog.V(4).Infof("We have multiple mapping for service proxy: %s, no need to destory yet", request.Proxier)
				serviceIDs[serviceID] -= 1
			}

		// We have received a service binding request
		case request := <-bindings:
			glog.V(4).Infof("Recieved a service bind request, request: %s", request)
			proxyID := GetProxyIDByService(&request.Service)
			proxy, found := proxyIDs[proxyID]

			switch request.Action {
			case services.SERVICE_REQUEST:
				// If a proxy was not found we need to create one, which we can do in the background
				if !found {
					go func() {
						// We create the service proxy and if everything we successful we send to create
						proxy, err := px.CreateProxyService(request.Service)
						if err != nil {
							glog.Errorf("Failed to create the proxy for service: %s, error: %s", request.Service, err)
							return
						}
						glog.V(4).Infof("Adding Service proxyId: %s, proxier: %s to services", proxyID, proxy)
						create <- ProxyDefinition{proxyID, proxy}
					}()
				} else {
					// A proxy already exists for this service, we simply add another mapping
					create <- ProxyDefinition{proxyID, proxy}
				}
			case services.SERVICE_REMOVAL:
				if found {
					destroy <- ProxyDefinition{proxyID, proxy}
				}
			}

		case <-px.shutdown:
			glog.Infof("Received shutdown signal. Propagating the signal to the proxies: %d", len(proxyIDs))
			for _, proxy := range proxyIDs {
				proxy.Close()
			}
		}
	}
	return nil
}

func (px *ProxyStore) CreateProxyService(si services.Service) (ServiceProxy, error) {
	// step: we need to create a new service proxy for this service
	glog.Infof("Creating new service proxy for service: %s", si)
	if proxy, err := NewServiceProxy(si); err != nil {
		glog.Errorf("Unable to create proxier, service: %s, error: %s", si, err)
		return nil, err
	} else {
		return proxy, nil
	}
}

/* Derive the original port from the tcp header */
func (px *ProxyStore) GetOriginalPort(conn *net.TCPConn) (string, error) {
	descriptor, err := conn.File()
	if err != nil {
		glog.Errorf("Unable to get tcp descriptor, connection: %s, error: ", conn.RemoteAddr(), err)
		return "", err
	}
	if addr, err := syscall.GetsockoptIPv6Mreq(int(descriptor.Fd()), syscall.IPPROTO_IP, SO_ORIGINAL_DST); err != nil {
		glog.Errorf("Unable to get the original destination port for connection: %s, error: %s", conn.RemoteAddr(), err)
		return "", err
	} else {
		destination := uint16(addr.Multiaddr[2])<<8 + uint16(addr.Multiaddr[3])
		return strconv.Itoa(int(destination)), nil
	}
}
