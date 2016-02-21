FROM alpine:3.1
MAINTAINER Dr. Stefan Schimanski <stefan.schimanski@gmail.com>

RUN apk add -U ca-certificates && rm -rf /var/cache/apk/*

COPY elastic-etcd /elastic-etcd

CMD ["/elastic-etcd", "--help"]
