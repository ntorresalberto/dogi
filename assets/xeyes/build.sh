#!/usr/bin/env bash
set -e

shopt -s expand_aliases
alias trace_on='set -x'
alias trace_off='{ set +x; } 2>/dev/null'
alias say='{ set +x; } 2>/dev/null; echo $1'

image=$(grep FROM Dockerfile | cut -d' ' -f2)

echo "- pull image: $image"
trace_on
# docker pull ${image}
trace_off

echo "- build image"
trace_on
docker build -t xeyes .
trace_off
