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
	"sync"
	"syscall"

	"github.com/gambol99/embassy/config"
	"github.com/gambol99/embassy/services"
	"github.com/golang/glog"
)

type ProxyService interface {
	ProxyServices() error
}

type ProxyStore struct {
	sync.RWMutex
	/* the tcp listener */
	Listener net.Listener
	/* a map of the proxy [source_ip+service_port] to the proxier handler */
	Proxies map[ProxyID]*Proxier
	/* channel for new service requests from the store */
	ServicesChannel services.ServiceStoreChannel
	/* channel for shutdown down */
	ShutdownSignal chan bool
	/* service configuration */
	Config *config.Configuration
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
	glog.V(4).Infof("Binding proxy to interface: %s:%d", cfg.IPAddress, cfg.ProxyPort)
	//tcp_addr := net.TCPAddr{net.ParseIP(cfg.IPAddress), cfg.ProxyPort, ""}
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.ProxyPort))
	if err != nil {
		glog.Errorf("Unable to bind proxy service, error: %s", err)
		return nil, err
	}
	proxy.Listener = listener

	/* step: create the map for holder proxiers */
	proxy.Proxies = make(map[ProxyID]*Proxier, 0)
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
	if err := px.ProxyConnnections(); err != nil {
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
					glog.Infof("Unable to create proxier, service: %s, error: %s", service, err)
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
	- Wait for connections on the tcp listenr
	- Get the original port before redirection
	- Lookup the service proxy for this service (src_ip + original_port)
	- Pass the connectinto the in a go handler
*/
func (px *ProxyStore) ProxyConnnections() error {
	go func() {
		for {
			/* wait for a connection */
			conn, err := px.Listener.Accept()
			if err != nil {
				glog.Errorf("Accept connection failed: %s", err)
				continue
			}
			/* step: get the destination port and source ip address */
			source_ipaddress, _, err := net.SplitHostPort(conn.RemoteAddr().String())
			if err != nil {
				glog.Errorf("Unable to get the remote ipaddress and port, error: %s", err)
				conn.Close()
			}
			/* step: get the original port */
			original_port, err := GetOriginalPort(conn.(*net.TCPConn))
			if err != nil {
				glog.Errorf("Unable to get the original port, connection: %s, error: %s", conn.RemoteAddr(), err)
				conn.Close()
				continue
			}

			glog.V(4).Infof("Accepted TCP connection from %v to %v, original port: %s", conn.RemoteAddr(), conn.LocalAddr(), original_port)
			/* step: create a proxyId for this */
			proxyId := GetProxyIDByConnection(source_ipaddress, original_port)
			/* step: find the proxier responsible for handling this connection */
			if proxier, found := px.LookupProxierByProxyID(proxyId); found {
				/* step: handle the connection in the proxier */
				go func(client *net.TCPConn) {
					if err := proxier.HandleTCPConnection(client); err != nil {
						glog.Errorf("Failed to handle the connection: %s, proxyid: %s, error: %s", client.RemoteAddr(), proxyId, err)
						if err := client.Close(); err != nil {
							glog.Errorf("Failed to close the connection connection: %s, error: %s", client.RemoteAddr(), err)
						}
					}
				}(conn.(*net.TCPConn))
			} else {
				glog.Errorf("Failed to handle service, we do not have a proxier for: %s", proxyId)
				conn.Close()
			}
		}
	}()
	return nil
}

func (px *ProxyStore) LookupProxierByProxyID(id ProxyID) (proxier *Proxier, found bool) {
	px.RLock()
	defer px.RUnlock()
	proxier, found = px.Proxies[id]
	return
}

func (px *ProxyStore) AddServiceProxier(proxier *Proxier) {
	px.Lock()
	defer px.Unlock()
	glog.V(3).Infof("Adding the proxyId: %s to collection of proxies: %d", proxier.ID, len(px.Proxies))
	px.Proxies[proxier.ID] = proxier
}

const SO_ORIGINAL_DST = 80

func GetOriginalPort(conn *net.TCPConn) (string, error) {
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
