#!/bin/bash
#
#   Author: Rohith (gambol99@gmail.com)
#   Date: 2014-11-21 16:48:39 +0000 (Fri, 21 Nov 2014)
#
#  vim:ts=2:sw=2:et
#

DISCOVERY=${DISCOVERY:-"etcd://localhost:4001"}
INTERFACE=${PROXY_INTERFACE:-eth0}

/bin/embassy -discovery ${DISCOVERY} -interface ${INTERFACE} -alsologtostderr=true -v 10 $@
