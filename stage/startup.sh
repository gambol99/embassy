#!/bin/bash
#
#   Author: Rohith (gambol99@gmail.com)
#   Date: 2014-11-21 16:48:39 +0000 (Fri, 21 Nov 2014)
#
#  vim:ts=2:sw=2:et
#

DISCOVERY=${DISCOVERY:-"etcd://localhost:4001"}
INTERFACE=${PROXY_INTERFACE:-eth0}
VERBOSITY=${VERBOSITY:-0}
PROXY_PORT="9999"
RULE_COMMENT="embassy_redirection"
RULE_CHAIN="PREROUTING"
IPTABLES="/sbin/iptables"

failed() {
  [ -n "$@" ] || {
    echo "Error: $@";
    exit 1
  }
}

ADD_REDIRECT_RULE="-t nat -A PREROUTING -p tcp -j REDIRECT --to-ports ${PROXY_PORT}"

iptables_setup() {
  # step: check if the rule is already there
  $IPTABLES $ADD_REDIRECT_RULE || failed "unable to add the iptables redirection rule"
}

# step: make sure we clean up afterwards
trap "delete_iptables_redirect" SIGTERM SIGHUP

echo "Adding the iptables redirection rule: $ADD_REDIRECT_RULE"
iptables_setup
echo "Iptables Prerouting Table"
$IPTABLES -t nat -L PREROUTING --line-numbers
echo "Starting the Embassy Proxy Service"
/bin/embassy -discovery ${DISCOVERY} -interface ${INTERFACE} -logtostderr=true -v ${VERBOSITY}
