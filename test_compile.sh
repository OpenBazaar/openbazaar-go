#!/bin/bash

set -e
pwd
go test -cover -coverprofile=api.cover.out ./api
go test -cover -coverprofile=bitcoin.cover.out ./bitcoin
go test -cover -coverprofile=bitcoin.listeners.cover.out ./bitcoin/listeners
go test -cover -coverprofile=core.cover.out ./core
go test -cover -coverprofile=ipfs.cover.out ./ipfs
go test -cover -coverprofile=mobile.cover.out ./mobile
go test -cover -coverprofile=net.cover.out ./net
go test -cover -coverprofile=net.service.cover.out ./net/service
go test -cover -coverprofile=net.repointer.cover.out ./net/repointer
go test -cover -coverprofile=net.retriever.cover.out ./net/retriever
go test -cover -coverprofile=repo.cover.out ./repo
go test -cover -coverprofile=repo.db.cover.out ./repo/db
go test -cover -coverprofile=repo.migrations.db.cover.out ./repo/migrations
go test -cover -coverprofile=schema.cover.out ./schema
go test -cover -coverprofile=storage.cover.out ./storage
go test -cover -coverprofile=storage.dropbox.cover.out ./storage/dropbox
go test -cover -coverprofile=storage.selfhosted.cover.out ./storage/selfhosted
echo "mode: set" > coverage.out && cat *.cover.out | grep -v mode: | sort -r | \
awk '{if($1 != last) {print $0;last=$1}}' >> coverage.out
rm -rf *.cover.out
