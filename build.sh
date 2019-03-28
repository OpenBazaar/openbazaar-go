#!/bin/bash

mkdir dist && cd dist/

if [[ "$TRAVIS_OS_NAME" != "osx" ]]; then
#    TARGETS=${1:-darwin/amd64}
    export GOBIN=$GOPATH/bin
    go install openbazaard.go
    mv $GOBIN/openbazaard dist/openbazaar-go-darwin-10.6-amd64
else
    TARGETS=${1:-linux/386,linux/amd64,linux/arm}
    export CGO_ENABLED=1
    docker pull karalabe/xgo-latest
    go get github.com/karalabe/xgo
    xgo -go=1.10 --targets=$TARGETS ../
    chmod +x *
fi
