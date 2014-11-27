#
#   Author: Rohith (gambol99@gmail.com)
#   Date: 2014-11-21 16:43:24 +0000 (Fri, 21 Nov 2014)
#
#  vim:ts=2:sw=2:et
#
FROM centos
MAINTAINER <gambol99@gmail.com>

ADD ./stage/embassy /bin/embassy
ADD ./stage/startup.sh ./startup.sh
#RUN apt-get update && apt-get install -y iptables
#RUN yum install -y iptables
RUN chmod +x /startup.sh; chmod +x /bin/embassy
EXPOSE 9999
ENTRYPOINT [ "/startup.sh" ]
