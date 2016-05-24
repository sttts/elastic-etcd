FROM gcr.io/google_containers/etcd:2.2.5
MAINTAINER Dr. Stefan Schimanski <stefan.schimanski@gmail.com>

COPY release/elastic-etcd /usr/local/bin/elastic-etcd

ADD certs.tar /
RUN mkdir -p /etc/ssl/certs && cd /etc/ssl/certs/ && ln -s /usr/share/ca-certificates/* .