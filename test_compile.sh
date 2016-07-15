#!/bin/bash

go test -coverprofile=api.cover.out -coverpkg=./api
go test -coverprofile=bitcoin.cover.out -coverpkg=./bitcoin
go test -coverprofile=bitcoinlibbitcoin.cover.out -coverpkg=./bitcoin/libbitcoin
go test -coverprofile=core.cover.out -coverpkg=./core
go test -coverprofile=ipfs.cover.out -coverpkg=./ipfs
go test -coverprofile=net.cover.out -coverpkg=./net
go test -coverprofile=netservice.cover.out -coverpkg=./net/service
go test -coverprofile=pb.cover.out -coverpkg=./pb
go test -coverprofile=repo.cover.out -coverpkg=./repo
go test -coverprofile=repodb.cover.out -coverpkg=./repo/db
go test -coverprofile=storage.cover.out -coverpkg=./storage
go test -coverprofile=dropbox.cover.out -coverpkg=./storage/dropbox
go test -coverprofile=selfhosted.cover.out -coverpkg=./storage/selfhosted
echo "mode: set" > coverage.out && cat *.cover.out | grep -v mode: | sort -r | \
awk '{if($1 != last) {print $0;last=$1}}' >> coverage.out
rm -rf *.cover.out
