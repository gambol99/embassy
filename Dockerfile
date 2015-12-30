FROM alpine:3.2
MAINTAINER <gambol99@gmail.com>

RUN apk update && \
    apk add ca-certificates

ADD ./bin/embassy /bin/embassy
ADD startup.sh ./startup.sh
RUN chmod +x /startup.sh && \
    chmod +x /bin/embassy 

ENTRYPOINT [ "/startup.sh" ]
