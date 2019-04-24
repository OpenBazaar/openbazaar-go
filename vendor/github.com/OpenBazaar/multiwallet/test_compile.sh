#!/bin/bash

set -e
pwd
go get gopkg.in/jarcoal/httpmock.v1
go get github.com/OpenBazaar/multiwallet
go get github.com/mattn/go-sqlite3
go test -coverprofile=bitcoin.cover.out ./bitcoin
go test -coverprofile=client.cover.out ./client
go test -coverprofile=config.cover.out ./config
go test -coverprofile=keys.cover.out ./keys
go test -coverprofile=litecoin.cover.out ./litecoin
go test -coverprofile=litecoin.address.cover.out ./litecoin/address
go test -coverprofile=service.cover.out ./service
go test -coverprofile=util.cover.out ./util
go test -coverprofile=zcash.cover.out ./zcash
go test -coverprofile=zcash.address.cover.out ./zcash/address
go test -coverprofile=multiwallet.cover.out ./
echo "mode: set" > coverage.out && cat *.cover.out | grep -v mode: | sort -r | \
awk '{if($1 != last) {print $0;last=$1}}' >> coverage.out
rm -rf *.cover.out
