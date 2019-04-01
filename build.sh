#!/bin/bash

TARGETS=${1:-windows/386,windows/amd64,darwin/amd64,linux/386,linux/amd64,linux/arm}

export CGO_ENABLED=1
go get github.com/karalabe/xgo
mkdir dist && cd dist/
xgo -go=1.11 --targets=$TARGETS ../
chmod +x *
