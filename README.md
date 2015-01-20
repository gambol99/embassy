
[![Build Status](https://drone.io/github.com/gambol99/embassy/status.png)](https://drone.io/github.com/gambol99/embassy/latest)

### **Embassy**
Is a service proxy / load balancer for docker container services, using etcd | consul for service endpoint discovery. Presently it can be run on the following modes; 

>   - run locally inside the container as seperate service
>   - (recommended) run on the docker host it self and use port mapping between host and container to permit the services
>   - run as in a seperater container and use links and iptables to bridge the connections

Embassy run is a single tcp port with iptables dnatting the destination and port over to the proxy from the virtual proxy ip (namely the docker0 interface ip)

#### **Service Providers**

At present embassy supports two service providers;

>  **Docker services**: we read the services and backend requests when a container is started. Note, we also perform a initial listing during startup (so anything already running and requesting backends is processed). Naturally, we need access to the docker socket to listen to events.

> **Static services**: the service requests are read from the command line when embassy is started up

#### **Docker Usage**

At present networking is perform in one of one two; if we are running the service proxy on the docker host, we'd have to DNAT *(i.e. --dnat)* between the containers and the parent host, alternatively if we are running the service with a container or using docker links we can use iptables redirect --redirect. Check the startup.sh in stage/  to see the code.

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

- You already have some means of service discovery, registering container services with a backend (take a look at [service-registrar](https://github.com/gambol99/service-registrar) if not)

        # docker run -d --privileged=true --net=host -e INTERFACE=[HOST-IFACED] -v \
        /var/run/docker.sock:/var/run/docker.sock -e DISCOVERY="etcd://HOST:4001" gambol99/embassy

When the docker boot is will create a iptables entry for DNAT all traffic from 172.17.42.1 to HOST_IFACE:9999. Check the stage/startup.sh if you wish to alter this

- Service discovery has registered mutiple containers for a service, say 'app1' in the backend

        /services/prod/app1/80/49173/e6d41829bd76   <- instance
        /services/prod/app1/80/49175
        /services/prod/app1/80/49175/9fb514731beb   <- instance
        /services/prod/app1/80/49177
        /services/prod/app1/80/49177/6b06da408f97   <- instance

- Now you want your frontend box to be connected with with app1

        # docker run -d -P BACKEND_APP1="/services/prod/app1/80;80" app1
        # curl 172.17.42.1

##### **Embassy will**;

> - see the container creation, read the environment variables, scan for service request/s
> - in this example, it create a proxier with the proxyID = container_ip + service_port
> - pull the endpoints from etcd
> - proxy any connections made to proxy:80 within app1 via a load balancer (default is round robin - or least connections) over to the endpoints.
> (Note: these ports are overlaying, thus another container is allowed to map another service to the same binding proxy:80 but can be redirected to a completely different place)
> - naturally, if the endpoints are changed, updated or removed the changes are propagated to the proxy


Alternative are run the proxy inside a container, link the other containers and use iptables to redirect ports

      # ./embassy -interface eth0 -discovery 'etcd://HOST:PORT' -v=0 -p=9999
      # iptables -t nat -I PREROUTING -p tcp -d 172.17.42.1 -j DNAT --to-destination HOST:3232

      OR on the docker0 interface directly

      # ./embassy -interface docker0 -discovery 'etcd://HOST:PORT' -v=0 -p=3232
      # iptables -t nat -I PREROUTING -p tcp -d 172.17.42.1 -j REDIRECT --to-ports 3232

Note: mutiple services are simple added by placing additional environment variables

      -e BACKEND_APP1="/services/prod/app1/80[prod,app1];80" \
      -e BACKEND_DB_SLAVES="/services/prod/db/slaves/3306;3306" \
      -e BACKEND_DB_MASTER="/services/prod/db/master/3306;3306"

---------------
#### **Service Descriptor**

Service descriptors are read from the environment variables of the container, they MUST be prefixed with 'BACKEND_' and the rest is up to you.

The descriptor itself has the following format;

    BACKEND_NAME=<SERVICE_NAME>;<PORT>

    consul example
    BACKEND_REDIS_SERVICE=redis.master;6379
    or using etcd keys
    BACKEND_REDIS_SERVICE=/services/prod/redis.master;6379

--------------

#### **QuickStart**

Take a look at wiki showing a [CoreOS](https://github.com/gambol99/embassy/tree/master/docs) example.

### **Discovery Agent**

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

Discovery will then read these and produce an endpoint of 192.168.13.90:49161

#### **Consul Notes**

The consul agent watches for changes on services; the manner in which you get the services registered once again is up to you, though take a look at [registrator](https://github.com/progrium/registrator) if you've not got something in place already.
    
      $ ./embassy -interface eth0 -discovery 'consul://HOST:8500' -v=3 -p=9999
      # (assuming registrator and a consul cluster is already at hand)
      $ docker run -d -P -e SERVICE_80_NAME=frontend_http eboraas/apache
      # linking the backend
      $ docker run -ti --rm -e BACKEND_APACHE_80='frontend_http;80' centos /bin/bash
      [e6d41829bd76] $ curl 172.17.42.1

