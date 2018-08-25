#!/bin/bash

set -e
pwd
go test -coverprofile=omnibus.cover \
  ./net/... \
  ./ipfs/... \
  ./mobile/... \
  ./schema/... \
  ./bitcoin/... \
  ./storage/...
go test -coverprofile=api.cover.out ./api
go test -coverprofile=core.cover.out ./core
go test -coverprofile=repo.cover.out ./repo
echo "mode: set" > coverage.out && cat *.cover.out | grep -v mode: | sort -r | \
awk '{if($1 != last) {print $0;last=$1}}' >> coverage.out
rm -rf *.cover.out
