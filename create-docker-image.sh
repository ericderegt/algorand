#!/bin/sh
set -E
docker build -t local/algorand-peer -f Dockerfile .
