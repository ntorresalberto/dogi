#!/usr/bin/env bash
set -e

docker run --tty -p 8000:8000 -p 3142:3142 \
       dogi/apt-cacher
