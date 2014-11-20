

Service Descriptor
==================

The service descriptor has the following format;

  NAME=<SERVICE_NAME>[OPTIONAL_TAGS,..];<PORT>

An example passed in as a environment variable container starting up would be;

  BACKEND_REDIS_SERVICE=redis.master[prod,stats];6379

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

