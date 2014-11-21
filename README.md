
Service Descriptor
==================

The service descriptor has the following format;

  NAME=<SERVICE_NAME>[OPTIONAL_TAGS,..];<PORT>/<PROTO>

An example passed in as a environment variable container starting up would be;

  BACKEND_REDIS_SERVICE=redis.master[prod,stats];6379/tcp
  or using etcd keys
  BACKEND_REDIS_SERVICE=/services/prod/redis.master[prod,stats];6379/tcp

Discovery Agent
===============

Etcd Notes
-----------

The discovery agent will recusively retrieve all nodes under the branch. An example registration given below

    /services/prod/apache/80/49173/e6d41829bd76   <- instance
    /services/prod/apache/80/49175
    /services/prod/apache/80/49175/9fb514731beb   <- instance
    /services/prod/apache/80/49177
    /services/prod/apache/80/49177/6b06da408f97   <- instance

The value of the key must be a json string which at the MINIMUM holds entries for "ipaddress" and "host_port" and potentially tags (the ip address of the docker host the container is running on and the port which the service is exposed)

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

Break down;

  - The BACKEND_[NAME] prefix simply provides a means to distingush the service requests from other environment variables
  - the service name is 'redis.master' is actual service name and is what the DiscoveryService used to lookup the service upon
  - the optional [prod,stats] is a comma seperated list of tags associated to the service. Consul for example allow us to associate tags to a service endpoint; if you are using another discovery services, its assumed you have created these tags when registering the service
  - the 6379 the port number the service will be available upon, i.e. localhost:6379 will proxy to the redis.master service

Service Provider
================

The job of the service provider is to provide us a list of services to create proxies for; The default provider does the following

  - Watches for docker events (namely containers starting up)
  - If a container has started, we check if the container is linked to our proxy (i.e. --net=container:x)
  - If yes, we pull the environment variables from the new container and check to see if their are any service links which was required .. i.e BACKEND_REDIS_MASTER=redis.master[prod,stats];6379
  - If yes, we pass the information to the DiscoveryService

Discovery Service
=================

The discovery service receives a service request, taking the above example; redis.master[prod,stats]

  - the service performs a one-time pull of the service endpoints
  - it then places a watch on the service key and passes updates via its channel

