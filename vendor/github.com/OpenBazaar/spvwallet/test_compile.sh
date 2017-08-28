#!/bin/bash

set -e
pwd
go get github.com/OpenBazaar/spvwallet
go get github.com/mattn/go-sqlite3
go test -coverprofile=spvwallet.cover.out ./
echo "mode: set" > coverage.out && cat *.cover.out | grep -v mode: | sort -r | \
awk '{if($1 != last) {print $0;last=$1}}' >> coverage.out
rm -rf *.cover.out
