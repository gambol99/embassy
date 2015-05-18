#
#   Author: Rohith (gambol99@gmail.com)
#   Date: 2014-11-21 16:43:24 +0000 (Fri, 21 Nov 2014)
#
#  vim:ts=2:sw=2:et
#
FROM gliderlabs/alpine:3.1
MAINTAINER <gambol99@gmail.com>

ADD ./stage/embassy /embassy
ADD ./stage/startup.sh ./startup.sh
RUN chmod +x /startup.sh && \
    chmod +x /embassy && \
    apk --update add iptables bash 

ENTRYPOINT [ "/startup.sh" ]
