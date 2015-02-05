#
#   Author: Rohith (gambol99@gmail.com)
#   Date: 2014-11-21 16:43:24 +0000 (Fri, 21 Nov 2014)
#
#  vim:ts=2:sw=2:et
#
FROM centos:centos7
MAINTAINER <gambol99@gmail.com>

ADD startup.sh ./startup.sh
RUN curl -ksL https://drone.io/github.com/gambol99/embassy/files/embassy.gz > /bin/embassy.gz
RUN md5sum /bin/embassy.gz
RUN gunzip /bin/embassy.gz
RUN chmod +x /startup.sh; chmod +x /bin/embassy
ENTRYPOINT [ "/startup.sh" ]
