#!/bin/bash
#
#   Author: Rohith (gambol99@gmail.com)
#   Date: 2014-11-21 16:48:39 +0000 (Fri, 21 Nov 2014)
#
#  vim:ts=2:sw=2:et
#
PROXY_IP="172.17.42.1"
PROXY_PORT="9999"
NETWORK_MODE="DNAT"
NETWORK_CHIAN="PREROUTING"

IPTABLES="/sbin/iptables"
IPTABLES_RULE_COMMENT="embassy_redirection"
IPTABLES_DNAT_RULE="-t nat -I PREROUTING -p tcp --dst ${PROXY_IP} -m comment --comment "${IPTABLES_RULE_COMMENT}" -j DNAT --to ${PROXY_IP}:${PROXY_PORT}"
IPTABLES_REDIRECT_RULE="-t nat -I PREROUTING -p tcp --src 0/0 -m comment --comment "${IPTABLES_RULE_COMMENT}" -j REDIRECT --to-ports ${PROXY_PORT}"

annonce() {
  echo "** $@"
}

failed() {
  [ -n "$@" ] && {
    annonce "Error: $@";
    exit 1
  }
}

iptables_rule_exists() {
  ${IPTABLES} -t nat -L ${NETWORK_CHIAN} | grep -q "${IPTABLES_RULE_COMMENT}" && return 0 || return 1
}

iptables_delete_rule() {
  annonce "Deleting the iptables rule: ${IPTABLES} ${IPTABLES_DELETE_RULE} number: ${RULENO}"
  # step: we dont care about the actual rule, just the comment which we add in the rule
  iptables_no=$(${IPTABLES} -t nat -L ${NETWORK_CHIAN} --line-numbers | awk "/${IPTABLES_RULE_COMMENT}/ { print \$1 }" | head -n1)
  # step: show the debug lines
  annonce "Deleting the iptables rule: ${IPTABLES} -t nat -L ${NETWORK_CHIAN} number: ${iptables_no}"
  # step: delete the actual rule from iptables
  ${IPTABLES} -t nat -D ${NETWORK_CHIAN} ${iptables_no} || failed "failed to remove the iptables rule"
}

iptables_delete_redirect_rule() {
  # step: we dont care about the actual rule, just the comment which we add in the rule
  RULENO=$(${IPTABLES} ${IPTABLES_HAS_REDIRECT_RULE} | awk "/${IPTABLES_RULE_COMMENT}/ { print \$1 }" | head -n1)
  annonce "Deleting the rule: ${IPTABLES} -D ${NETWORK_CHIAN} number: ${RULENO}"
  # step: delete the actual rule from iptables
  ${IPTABLES} -D ${NETWORK_CHIAN} ${RULENO} || failed "failed to remove the iptables rule"
}

iptables_add_rule() {
  annonce "Adding the iptables rule, mode: ${NETWORK_MODE}, chain: ${NETWORK_CHIAN}, proxy: ${PROXY_IP}:${PROXY_PORT}"
  case ${NETWORK_MODE} in
  DNAT) ${IPTABLES} ${IPTABLES_DNAT_RULE} || failed "unable to add the iptables rule"
        ;;
  REDIRECT)
        ${IPTABLES} ${IPTABLES_REDIRECT_RULE} || failed "unable to add the iptables rule"
        ;;
  *)    failed "unknown networking mode: ${NETWORK_CHIAN}"
        ;;
  esac
}

iptables_show() {
  annonce "Iptables Rules"
  ${IPTABLES} -t nat -L ${NETWORK_CHIAN} --line-numbers
}

setup_iptables() {
  annonce "Setting up the iptables rule set for embassy proxy"
  # step: we check if the rule exists
  if iptables_rule_exists; then
    # step: we delete the rule
    iptables_delete_rule
  fi
  # step: we then add back the iptables rule
  iptables_add_rule
}

case $1 in
  --dnat)     NETWORK_MODE="DNAT"
              NETWORK_CHIAN="PREROUTING"

              ;;
  --redirect) NETWORK_MODE="REDIRECT"
              NETWORK_CHIAN="OUTPUT"
              shift 1
              ;;
  *)          shift
              ;;
esac

# step; we first need to setup iptables
setup_iptables
# step: for debugging purposes lets show the table
iptables_show
# step: lets start the embassy proxy
annonce "Starting the Embassy Services Proxy"
/bin/embassy -logtostderr=true $@
