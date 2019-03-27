#!/bin/bash

if [[ "$TRAVIS_OS_NAME" != "osx" ]]; then
    TARGETS=${1:-darwin/amd64}
else
    TARGETS=${1:-linux/386,linux/amd64,linux/arm}
fi

export CGO_ENABLED=1
docker pull karalabe/xgo-latest
go get github.com/karalabe/xgo
mkdir dist && cd dist/
xgo -go=1.10 --targets=$TARGETS ../
chmod +x *
