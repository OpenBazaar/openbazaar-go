#!/bin/bash

pwd
go test -coverprofile=api.cover.out ./api
go test -coverprofile=api.cover.out ./api/notifications
go test -coverprofile=bitcoin.cover.out ./bitcoin
go test -coverprofile=bitcoin.cover.out ./bitcoin/exchange
go test -coverprofile=bitcoin.cover.out ./bitcoin/listeners
go test -coverprofile=core.cover.out ./core
go test -coverprofile=ipfs.cover.out ./ipfs
go test -coverprofile=net.cover.out ./net
go test -coverprofile=netservice.cover.out ./net/service
go test -coverprofile=netservice.cover.out ./net/repointer
go test -coverprofile=netservice.cover.out ./net/retriever
go test -coverprofile=repo.cover.out ./repo
go test -coverprofile=repodb.cover.out ./repo/db
go test -coverprofile=storage.cover.out ./storage
go test -coverprofile=dropbox.cover.out ./storage/dropbox
go test -coverprofile=selfhosted.cover.out ./storage/selfhosted
echo "mode: set" > coverage.out && cat *.cover.out | grep -v mode: | sort -r | \
awk '{if($1 != last) {print $0;last=$1}}' >> coverage.out
rm -rf *.cover.out

for SCRIPT in ~/qa/*
do
   python3 $SCRIPT -b $GOPATH/bin/openbazaar-go
done
