FROM gcr.io/google_containers/etcd:2.2.1
MAINTAINER Dr. Stefan Schimanski <stefan.schimanski@gmail.com>

COPY release/elastic-etcd /usr/local/bin/elastic-etcd
