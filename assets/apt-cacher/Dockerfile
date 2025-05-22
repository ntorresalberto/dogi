# https://docs.docker.com/samples/apt-cacher-ng/
# Build: docker build -t apt-cacher .
# Run: docker run -d -p 3142:3142 --name apt-cacher-run apt-cacher
#
# and then you can run containers with:
#   docker run -t -i --rm -e http_proxy http://dockerhost:3142/ debian bash
#
# Here, `dockerhost` is the IP address or FQDN of a host running the Docker daemon
# which acts as an APT proxy server.

# add this to container:
# echo 'Acquire::http { Proxy "http://localhost:3142"; };' >> /etc/apt/apt.conf.d/01proxy
# echo 'Acquire::http { Proxy "http://172.17.0.2:3142"; };' >> /etc/apt/apt.conf.d/01proxy
FROM ubuntu:22.04

VOLUME ["/var/cache/apt-cacher-ng"]

## Added : Try to clean everything (for bug : apt-get BADSIG GPG)
RUN apt-get clean && \
    cd /var/lib/apt &&\
    mv lists lists.old &&\
    mkdir -p lists/partial && \
    apt-get clean &&\
    apt-get update -qq -y

RUN    apt-get update && apt-get install -y apt-cacher-ng \
    && sed -i 's/\# PassThroughPattern: \.\*/PassThroughPattern: \.\*/g' /etc/apt-cacher-ng/acng.conf

EXPOSE 3142
CMD    chmod 777 /var/cache/apt-cacher-ng && /etc/init.d/apt-cacher-ng start && tail -f /var/log/apt-cacher-ng/*
