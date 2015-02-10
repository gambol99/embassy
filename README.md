
[![Build Status](https://drone.io/github.com/gambol99/embassy/status.png)](https://drone.io/github.com/gambol99/embassy/latest)

### **Embassy**

Is a service proxy | load balancer for docker container services, using etcd | consul | marathon for service endpoint discovery. Presently it can be run on the following modes; 

>   - run locally inside the container as seperate service
>   - (recommended) run on the docker host it self and use port mapping between host and container to permit the services
>   - run as in a seperater container and use links and iptables to bridge the connections

Embassy runs on a 'single' tcp port (default 9999) with iptables dnatting/redirecting virtual ip (the docker0 bridge ip by default) traffic and load balancing the traffic to the backends.  

#### **Service Endpoint Discovery**
------

Embassy presently supports the following providers to pull endpoints from. Note; how you get your endpoints **into** them is up to you, though examples are referenced throughout this README.

>   - [Consul](https://consul.io)
>   - [Etcd](https://github.com/coreos/etcd)
>   - [Marathon](http://mesosphere.com)

#### **Service Providers**

At present embassy supports two service providers;

> **Docker services**: reads the services and backend requests from the environment variables when a container is started . Note, an initial listing is taken during startup, so anything already running and requesting backends is processed.

> **Static services**: the service requests are read from the command line when embassy is started up

#### **Docker Usage**
------

At present networking is perform in one of two ways; if we are running the service proxy on the docker host, we'd have to DNAT *(i.e. --dnat)* between the containers and the parent host, alternatively if we are running the service with a container or using docker links we can use iptables redirect --redirect. Check the startup.sh in stage/  to see the code.

      # a) Running the service proxy on the docker host itself
      #
      $ docker run -d --privileged=true --net=host \
        -e PROXY_IP=172.17.42.1 \
        -e PROXY_PORT=9999 \
        -v /var/run/docker.sock:/var/run/docker.sock \
        gambol99/embassy \
        --dnat -provider=docker -v=3 -interface=eth0 \
        -discovery=consul://HOST:8500

      # b) Running inside the container along with your services
      # (so you could run this under supervisord or bluepill etc)
      # Note: you'll need to add your own iptables rule to perform the redirect (see the /stage/startup.sh for details)
      $ embassy -discovery=consul://<IP>:8500 \
        -provider=static \
        -services='frontend_http;80,mysql;3306,redis;6563'

      # c) Run as a container and using docker links to proxy (docker or static providers)
      $ docker run -d --privileged=true \
        --name embassy \
        -e PROXY_IP=172.17.42.1 \
        -e PROXY_PORT=9999 \
        -v /var/run/docker.sock:/var/run/docker.sock \
        gambol99/embassy \
        --redirect -provider=docker -v=3 -interface=eth0 \
        -discovery=consul://HOST:8500

      # Link a container
      # docker run -ti --rm --links embassy:proxy -e BACKEND_FRONTEND='frontend_http;80' centos /bin/bash

#### **Example Usage**

- You already have some means of service discovery, registering container services with a backend (take a look at [service-registrar](https://github.com/gambol99/service-registrar) or [registrator](https://github.com/progrium/registrator) if not)

        # docker run -d --privileged=true --net=host 
        -v /var/run/docker.sock:/var/run/docker.sock \
        gambol99/embassy \
        --dnat -provider=docker \
        -v=3 -interface=eth0 \
        -discovery=etcd://HOST:4001

When the docker boots it will create a iptables entry for DNAT all traffic from 172.17.42.1 to HOST_IFACE:9999. Check the stage/startup.sh if you wish to alter this and the command line options.

- Service discovery has registered mutiple containers for a service, say 'app1' in the backend

            /services/prod/frontend/frontend_http/31000
            /services/prod/frontend/frontend_http/31000/47861e964ca5 <-instance
            /services/prod/frontend/frontend_http/31000/67c6fccb40d0 <-instance
            /services/prod/frontend/frontend_http/31000/cd52b6deca96 <-instance
            /services/prod/frontend/frontend_http/31002
            /services/prod/frontend/frontend_https
            /services/prod/frontend/frontend_https/31001
            /services/prod/frontend/frontend_https/31001/47861e964ca5 <-instance
            /services/prod/frontend/frontend_https/31001/67c6fccb40d0 <-instance
            /services/prod/frontend/frontend_https/31001/cd52b6deca96 <-instance
            /services/prod/frontend/frontend_https/31003

- Now you want your frontend box to be connected with with app1 on port 80

            # docker run -d -P BACKEND_APP1="/services/prod/frontend/frontend_http;80" app1
            # curl 172.17.42.1 
            <html><body><h1>It works!</h1>
            <p>This is the default web page for this server.</p>
            <p>The web server software is running but no content has been added, yet.</p>
            </body></html>

##### **Embassy will**;

> - see the creation of the container, read the environment variables, scan for service request/s
> - in this example above, create a proxier with the proxyID = container_ip + service_port
> - pull the endpoints from etcd
> - proxy any connections made to proxy:80 within app1 via a load balancer (default is round robin - or least connections) over to the endpoints.
> (Note: these ports are overlaying, thus another container is allowed to map another service to the same binding proxy:80 but can be redirected to a completely different place)
> - naturally, if the endpoints are changed, updated or removed the changes are propagated to the proxy

Note: mutiple services are simply added by placing additional environment variables

      -e BACKEND_APP1="/services/prod/app1/80[prod,app1];80" \
      -e BACKEND_DB_SLAVES="/services/prod/db/slaves/3306;3306" \
      -e BACKEND_DB_MASTER="/services/prod/db/master/3306;3306"

#### **Service Descriptor**
-------

Service descriptors are read from the environment variables of the container; by default these must be prefixed with BACKEND_ though you can change this on the command line.

The descriptor itself has the following format;

    PREFIX_NAME=<SERVICE_NAME>;<PORT>

    consul
    BACKEND_REDIS_SERVICE=redis.master;6379
    etcd
    BACKEND_REDIS_SERVICE=/services/prod/redis.master;6379
    marathon
    BACKEND_REDOES_SERVICE=/prod/redis.master/6379;6379  # view the notes for Marathon

#### **QuickStart**
-------

Take a look at the documentation showing a [CoreOS](https://github.com/gambol99/embassy/tree/master/docs) usage for a more complete example.

### **Discovery Agents**
-------

#### **Etcd Notes**

The etcd discovery agent will recursively retrieve all nodes under the branch. An example registration given below

    /services/prod/apache/80/49173/e6d41829bd76   <- instance
    /services/prod/apache/80/49175
    /services/prod/apache/80/49175/9fb514731beb   <- instance
    /services/prod/apache/80/49177
    /services/prod/apache/80/49177/6b06da408f97   <- instance

The value of the key must be a json string which holds entries for "ipaddress" and "host_port" (the ip address of the docker host the container is running on and the port which the service is exposed)at a minimum and potentially tags

An example of the service document (i.e. the etcd ) /services/prod/apache/80/49177/6b06da408f97

    {
      "id": "13532987eae2831eb7d41f38172cb82db668b3c72d10f513551026b2525e634f",
      "host": "fury",
      "ipaddress": "192.168.13.90",
      "env": {
        "ENVIRONMENT": "prod",
        "NAME": "apache",
        "HOME": "/",
        "PATH": "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
        "APACHE_RUN_USER": "www-data",
        "APACHE_RUN_GROUP": "www-data",
        "APACHE_LOG_DIR": "/var/log/apache2"
      },
      "tags": [],
      "name": "/trusting_shockley",
      "image": "eboraas/apache",
      "hostname": "13532987eae2",
      "host_port": "49161",
      "proto": "tcp",
      "port": "80",
      "path": "/services/prod/apache/80/49161/13532987eae2"
    }

Discovery will then read these and produce an endpoint of 192.168.13.90:49161.

##### **Etcd Certificates**

You can use the etcd tls support by passing the paths for the key, cert and cacert on the command line options

      -etcd-cacert="": the etcd ca certificate file (optional)
      -etcd-cert="": the etcd certificate file (optional)
      -etcd-keycert="": the etcd key certificate file (optional)

#### **Consul Notes**

The consul agent watches for changes on catalog services; the manner in which you get the services registered once again is up to you, though take a look at [registrator](https://github.com/progrium/registrator) if you've not got something in place already. Note, the endpoints filtered so only those passing the health checks are returned.
    
      $ ./embassy -interface eth0 -discovery 'consul://HOST:8500' -v=3 -p=9999
      # (assuming registrator and a consul cluster is already at hand)
      $ docker run -d -P -e SERVICE_80_NAME=frontend_http eboraas/apache
      # linking the backend
      $ docker run -ti --rm -e BACKEND_APACHE_80='frontend_http;80' centos /bin/bash
      [e6d41829bd76] $ curl 172.17.42.1

#### **Marathon Notes** 

In order to use Marathon as a service discovery provider you need to enable the events callback service via [--event_subscriber http_callback](http://mesosphere.github.io/marathon/docs/event-bus.html), which is obviously accessible by the docker host embassy is running on (Honestly!, someone did this!). Embassy will register itself as a callback with Marathon on default port of 10001 (though you can change this via the command line options).

The service definitions for service binding is somewhat different when using Marathon due to the nature of how marathon represents the applications internally. Where as Consul and the Etcd (albeit via the service-registrar / registrator in this case) divides the services by port, Marathon has no notion of this. A service / application in Marathon has port mappings for a selection of ports, but there is no means divide one from the other by name alone. Thus in the service definition has to be extended to allow us to explicitly state the *container* port (not the dynamic port) which we wish to proxy to. Example;


        # grab the tasks from the application product/web/frontend - which exposes ports 80 & 443
        # curl http://10.241.1.71:8080/v2/apps/product/web/frontend/tasks

        {
          "tasks": [
            {
              "appId": "/product/web/frontend",
              "id": "product_web_frontend.62e0f275-ac83-11e4-97af-ca6bcbbf53a9",
              "host": "10.241.1.72",
              "ports": [
                31000,
                31001
              ],
              "startedAt": "2015-02-04T15:35:03.892Z",
              "stagedAt": "2015-02-04T15:35:02.757Z",
              "version": "2015-02-04T15:35:00.811Z",
              "servicePorts": [
                80,
                443
              ]
            },
            {
              "appId": "/product/web/frontend",
              "id": "product_web_frontend.62e0a453-ac83-11e4-97af-ca6bcbbf53a9",
              "host": "10.241.1.61",
              "ports": [
                31000,
                31001
              ],
              "startedAt": "2015-02-04T15:35:04.484Z",
              "stagedAt": "2015-02-04T15:35:02.754Z",
              "version": "2015-02-04T15:35:00.811Z",
              "servicePorts": [
                80,
                443
              ]
            }
          ]
        }

 So in order to create a proxy to say port 80 we'd add the /PORT_NUMBER to the end of the service definition.

        BACKEND_FRONTEND_HTTP=/product/web/frontend/80;8080

        # i.e. map me 172.17.42.1:8080 -> |ENDPOINTS|:80
