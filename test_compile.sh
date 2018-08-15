#!/bin/bash

set -e
pwd
go test -coverprofile=api.cover.out ./api
go test -coverprofile=bitcoin.cover.out ./bitcoin
go test -coverprofile=bitcoin.listeners.cover.out ./bitcoin/listeners
go test -coverprofile=core.cover.out ./core
go test -coverprofile=ipfs.cover.out ./ipfs
go test -coverprofile=mobile.cover.out ./mobile
go test -coverprofile=net.cover.out ./net
go test -coverprofile=net.service.cover.out ./net/service
go test -coverprofile=net.repointer.cover.out ./net/repointer
go test -coverprofile=net.retriever.cover.out ./net/retriever
go test -coverprofile=repo.cover.out ./repo
go test -coverprofile=repo.db.cover.out ./repo/db
go test -coverprofile=repo.migrations.db.cover.out ./repo/migrations
go test -coverprofile=schema.cover.out ./schema
go test -coverprofile=storage.cover.out ./storage
go test -coverprofile=storage.dropbox.cover.out ./storage/dropbox
go test -coverprofile=storage.selfhosted.cover.out ./storage/selfhosted
echo "mode: set" > coverage.out && cat *.cover.out | grep -v mode: | sort -r | \
awk '{if($1 != last) {print $0;last=$1}}' >> coverage.out
rm -rf *.cover.out
