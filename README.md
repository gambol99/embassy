
Embassy
==========
Is a service proxy for docker containers, which uses either etcd/consul for service endpoint discovery. It can be run in the following modes;

>   - run locally inside the container as seperate service
>   - run on the docker host it self and use port mapping between host and container to permit the services
>   - (recommended) run as network container for one or more application containers

------------
Example Usage
-------------

- You already have some means service discovery, registering container services with a backend (take a look at [service-registrar](https://github.com/gambol99/service-registrar) or [registrator](https://github.com/progrium/registrator) if not)

        # docker run -d -P -e DISCOVERY="etcd://HOST:4001" gambol99/embassy

- Service discovery has registered mutiple containers for a service, say 'app1' in the backend

        /services/prod/app1/80/49173/e6d41829bd76   <- instance
        /services/prod/app1/80/49175
        /services/prod/app1/80/49175/9fb514731beb   <- instance
        /services/prod/app1/80/49177
        /services/prod/app1/80/49177/6b06da408f97   <- instance

- Now you want your frontend box to be connected with with app1

        # docker run -d -P -e BACKEND_APP1="/services/prod/app1/80[prod,app1];80/tcp" -net=container:<EMBASSY CONTAINER> app1

**Embassy will**;

> - see the container creation, read the environment variables, scan for service request/s
> - in this example whip up a proxy bound to 127.0.0.1:80 within app1 container
> - pull the endpoints from etcd
> - proxy any connections made to 127.0.0.1:80 within app1 via a load balancer (default is round robin - or least connections) over to the
> endpoints.
> - naturally, if the endpoints are changed, updated or removed the changes are propagated to the proxy

Note: mutiple services are simple added by placing additional environment variables

      -e BACKEND_APP1="/services/prod/app1/80[prod,app1];80/tcp" \
      -e BACKEND_DB_SLAVES="/services/prod/db/slaves/3306;3306/tcp" \
      -e BACKEND_DB_MASTER="/services/prod/db/master/3306;3306/tcp"

---------------
Service Descriptor
----------------------
Service descriptors are read from the environment variables of the container, they MUST be prefixed with 'BACKEND_' and the rest is up to you.

The descriptor itself has the following format;

    BACKEND_NAME=<SERVICE_NAME>[OPTIONAL_TAGS,..];<PORT>/<PROTO>

    consul example
    BACKEND_REDIS_SERVICE=redis.master[prod,stats];6379/tcp
    or using etcd keys
    BACKEND_REDIS_SERVICE=/services/prod/redis.master[prod,stats];6379/tcp

--------------
Discovery Agent
===============

Etcd Notes
-----------

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

Consul Notes
-------------
Provider still needs to be completed

