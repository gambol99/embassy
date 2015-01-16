# CoreOS Vagrant Demo

An example setup for testing / demoing the Embassy Proxy

***
## Installation


----------


  # Clone the https://github.com/gambol99/coreos-vagrant
  $ git clone https://github.com/gambol99/coreos-vagrant

    # Whip up the CoreOS cluster - *note, change instance number in the config/coreos-config.rb file*
    $ vagrant up

    # We should have (by default) 3 new CoreOS machines in a cluster.

  $ export FLEETCTL_ENDPOINT=http://10.0.1.101:4001
  $ fleetctl list-machines
  MACHINE   IP    METADATA
  0c7fbb04... 10.0.1.101  -
  13858cd5... 10.0.1.103  -
  e115fa6a... 10.0.1.102  -

  # Note: on occasion and i'm not sure why, the etcd discovery process doesn't always work. The easiest way to fix this is to run the cloudinit process manually off the command line. So if not all or any of the machines listed in the above output.

  $ vagrant ssh <machine_name>
  $ /usr/bin/coreos-cloudinit --from-file=/var/lib/coreos-vagrant/vagrantfile-user-data
  # Check that fleetd and etcd are running now
  $ ps aux | grep etcd
  $ fleetctl list-machines


----------

Services
--------

There are a number of CoreOS service units in the [coreos-vagrant](https://github.com/gambol99/coreos-vagrant) repository, located under services/ directory. Before we can use  the proxy we need some mean of service registration.
For the purpose of the demo we'll be using [registrator](https://github.com/progrium/registrator) agent.

    # Create the service discovery / registration service
    $ cd coreos-vagrant
    $ cd services
    $ cat registrator.service
    # Start the registration service

  # Start up Consul master
  $ fleetctl start consul-master.service
  # Wait for the master to come online and then startup the slaves
  $ fleetctl start consul-slave@[12].service

    UNIT                    MACHINE                 ACTIVE  SUB
  consul-master.service   5482a87e.../10.0.1.102  active  running
  consul-slave@1.service  ce96f1bc.../10.0.1.103  activ   running
  consul-slave@2.service  d293ec66.../10.0.1.101  active  running

  # We can now startup the registrator agent. Check the DISCOVERY environment variable in registrar.service to make sure it's still on consul://${COREOS_PRIVATE_IP}:8500, as i'm forever switching between that and etcd backend

    $ fleetctl start registrator.service

  # Wait for the service to come online
    $ watch -n1 -d fleetctl list-units
    UNIT      MACHINE     ACTIVE  SUB
    consul-master.service   5482a87e.../10.0.1.102  active  running
  consul-slave@1.service  ce96f1bc.../10.0.1.103  activ   running
  consul-slave@2.service  d293ec66.../10.0.1.101  active  running
  registrator.service     0c7fbb04.../10.0.1.101  active  running
  registrator.service     13858cd5.../10.0.1.103  active  running
  registrator.service     e115fa6a.../10.0.1.102  active  running

  # You should see x number of the registrar service running. If i've experienced issues or the service is in the failed state; login to a coreos machine and
  $ journalctl -u registrator.service
  # Or docker logs <containerId> the logs

  # You should be able to see the consul ui on http://10.0.1.101:8500

    # We can now push the Embassy Proxy service
  $ cat embassy.service
  $ fleetctl start embassy.service
  # -- and again wait on the fleetclt list-units until service is up


----------

## Testing ##

  # Now lets push some services in to the cluster
  $ for i in {1..3}; do fleetctl start apache@${i}.service; done
  # Wait for the services to come online

  # Note you can check the progress of the proxy on any of the CoreOS machines via using;
  $ journalctl -u embassy.service -f

    # Test off the command line by whipping up a container that requires backend services. You can see the service key via etcdctl (take a look at the service-registrar project to get a better understanding)
    $ docker run -ti --rm -e ENVIRONMENT=prod -e NAME=test -e BACKEND_APACHE_80='apache_http;80' centos /bin/bash

    [root@e53000208e81 /]# curl 172.17.42.1/hostname.php; echo
  66315dcbcf0d
  [root@e53000208e81 /]# curl 172.17.42.1/hostname.php; echo
  02a9c048f39e
  [root@e53000208e81 /]# curl 172.17.42.1/hostname.php; echo
  b69aae7729d4
  [root@e53000208e81 /]# curl 172.17.42.1/hostname.php; echo
  4cf161d53765
  [root@e53000208e81 /]# curl 172.17.42.1/hostname.php; echo
  66315dcbcf0d

  # You should see the proxy is round-robining the requests across the service instances.
  # Testing the performance
  $ yum install -y httpd-tools
  $ [root@e53000208e81 /]# ab -n 1000 -c 20 172.17.42.1/hostname.php
  ...
  Benchmarking 172.17.42.1 (be patient)
  Completed 100 requests
  Completed 200 requests
  Completed 300 requests
  Completed 400 requests
  Completed 500 requests
  Completed 600 requests
  Completed 700 requests
  Completed 800 requests
  Completed 900 requests
  Completed 1000 requests
  Finished 1000 requests

  ...
  Total transferred:      222000 bytes
  HTML transferred:       12000 bytes
  Requests per second:    1821.37 [#/sec] (mean)
  Time per request:       10.981 [ms] (mean)
  Time per request:       0.549 [ms] (mean, across all concurrent requests)
  Transfer rate:          394.87 [Kbytes/sec] received

  # If you want to multiple backend services simple add multiple -e BACKEND_NAME=<key> environment variables or place then in the Dockerfile









