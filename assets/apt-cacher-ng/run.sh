#!/usr/bin/env bash
set -e

docker run -d -p 3142:3142 --name test_apt_cacher_ng eg_apt_cacher_ng
