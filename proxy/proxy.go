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

type ProxyService interface {
	/* shutdown the proxy service */
	Close()
	/* start handling the events */
	Start() error
}

type ProxyStore struct {
	/* the tcp listener */
	Listener net.Listener
	/* a map of the proxy [source_ip+service_port] to the proxy service handler */
	Services ServiceMap
	/* channel for shutdown down */
	Shutdown utils.ShutdownSignalChannel
	/* the services store */
	Store services.ServiceStore
}

/* Create the proxy service - the main routine for handling requests and events */
func NewProxyService(store services.ServiceStore) (ProxyService, error) {
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
	service.Store = store
	service.Services = NewProxyServices()
	service.Shutdown = make(utils.ShutdownSignalChannel)
	service.Listener = listener
	return service, nil
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
	service_updates := make(services.ServiceEventsChannel, 5)
	defer close(service_updates)

	px.Store.AddServiceListener(service_updates)
	/* step: we need to start listening for services */
	if err := px.Store.Start(); err != nil {
		glog.Errorf("Failed to start the service discovery subsystem, error: %s", err)
		return err
	}

	/* step: start the connection handling */
	if err := px.ProxyConnections(); err != nil {
		glog.Errorf("Failed to start listening for connections, error: %s", err)
		return err
	}

	/*
		step: the main event loop for the proxy service;
		  - listening for service events
		  - listening for shutdown requests
	*/
	for {
		select {
		case <-px.Shutdown:
			glog.Infof("Received shutdown signal. Propagating the signal to the proxies: %d", px.Services.Size())
			for _, proxier := range px.Services.ListProxies() {
				glog.Infof("Sending the kill signal to proxy: %s", proxier)
				proxier.Close()
			}
		case event := <-service_updates:
			glog.V(3).Infof("ProxyService recieved service update, service: %s, action: %s", event.Service, event.Action)
			/* step: check if the service is already being processed */
			if err := px.ProcessServiceEvent(&event); err != nil {
				glog.Errorf("Unable to process the service request: %s, error: %s", event.Service, err)
			}
		}
	}
	return nil
}

func (px *ProxyStore) ProcessServiceEvent(event *services.ServiceEvent) error {
	/* check: is this a service request or down */
	if event.IsServiceRequest() {
		go px.Services.CreateServiceProxy(event.Service)
	} else if event.IsServiceRemoval() {
		go px.Services.DestroyServiceProxy(event.Service)
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
	glog.V(5).Infof("Starting to listen for incoming connections")

	go func() {
		defer func() {
			// I'm not sure about this???
			if panic := recover(); panic != nil && REAL_PANIC {
				fmt.Println("Recovered from Embassy panic: %s", panic)
			}
		}()
		for {
			/* step: wait for a connection */
			conn, err := px.Listener.Accept()
			if err != nil {
				glog.Errorf("Failed to accept connection, error: %s", err)
				continue
			}
			/* step: handle the rest with in go routine and return to pick up another connection */
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
				if proxier, found := px.Services.LookupProxyByProxyID(proxyId); found {

					/* step: handle the connection in the service proxy */
					if err := proxier.HandleTCPConnection(connection); err != nil {
						glog.Errorf("Failed to handle the connection: %s, proxyid: %s, error: %s", connection.RemoteAddr(),
							proxyId, err)
						if err := connection.Close(); err != nil {
							glog.Errorf("Failed to close the connection connection: %s, error: %s", connection.RemoteAddr(), err)
						}
					}
				} else {
					glog.Errorf("Failed to handle service, we do not have a proxier for: %s", proxyId)
					connection.Close()
				}
			}(conn.(*net.TCPConn))
		}
	}()
	return nil
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
