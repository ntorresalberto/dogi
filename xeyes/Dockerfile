FROM ubuntu

RUN apt-get update \
    && DEBIAN_FRONTEND=noninteractive apt-get upgrade -y \
    && DEBIAN_FRONTEND=noninteractive apt-get install -y \
    x11-apps \
    && apt-get autoremove -y && rm -rf /var/lib/apt/lists/*

CMD xeyes
