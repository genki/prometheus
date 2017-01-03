#!/bin/bash
rm -f prometheus
docker run --rm -it -v $(PWD):/go/src/github.com/prometheus/prometheus \
  -w /go/src/github.com/prometheus/prometheus -e CGO_ENABLED=0 golang \
  make build

docker build -t prometheus .
