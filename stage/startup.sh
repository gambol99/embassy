#!/bin/bash
#
#   Author: Rohith (gambol99@gmail.com)
#   Date: 2014-11-21 16:48:39 +0000 (Fri, 21 Nov 2014)
#
#  vim:ts=2:sw=2:et
#

DISCOVERY=${DISCOVERY:-"etcd://localhost:4001"}
INTERFACE=${INTERFACE:-eth0}
VERBOSITY=${VERBOSITY:-0}
PROXY_PORT="9999"
RULE_COMMENT="embassy_redirection"
RULE_CHAIN="PREROUTING"
IPTABLES="/sbin/iptables"
DOCKER_IP="172.17.42.1"

say() {
  echo "** $@"
}

failed() {
  [ -n "$@" ] || {
    say "Error: $@";
    exit 1
  }
}

get_ipaddress() {
  echo $(ip addr show $INTERFACE | awk '/inet/ { split($2,a,"/"); print a[1] }')
}

IPADDRESS=$(get_ipaddress)
DEL_RULE="-t nat -D PREROUTING "
HAS_RULE="-t nat -L PREROUTING --line-numbers"
ADD_RULE="-t nat -I PREROUTING -p tcp -d $DOCKER_IP -m comment --comment "$RULE_COMMENT" -j DNAT --to ${IPADDRESS}:${PROXY_PORT}"

rule_exists() {
  $IPTABLES $HAS_RULE | grep -q "${RULE_COMMENT}" && return 0 || return 1
}

rule_deletion() {
  RULENO=$($IPTABLES $HAS_RULE | awk "/$RULE_COMMENT/ { print \$1 }" | head -n1)
  echo "Deleting the rule: $IPTABLES $DEL_RULE $RULENO"
  $IPTABLES $DEL_RULE $RULENO || failed "unable to remove the rule"
}

rule_insert() {
  say "Adding the redirection rule for proxy: $IPTABLES $ADD_RULE"
  $IPTABLES $ADD_RULE || failed "unable to add the iptables redirection rule"
}

iptables_setup() {
  rule_exists && rule_deletion
  rule_insert
}

say "Adding the iptables redirection rule: $ADD_REDIRECT_RULE"
iptables_setup
say "Iptables Prerouting Table"
$IPTABLES -t nat -L PREROUTING --line-numbers
say "Starting the Embassy Proxy Service"
/bin/embassy -discovery ${DISCOVERY} -interface ${INTERFACE} -logtostderr=true -v=${VERBOSITY}
