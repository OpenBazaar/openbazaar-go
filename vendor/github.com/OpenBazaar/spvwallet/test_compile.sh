#!/bin/bash

set -e
pwd
go test -coverprofile=db.cover.out ./db
go test -coverprofile=spvwallet.cover.out ./
echo "mode: set" > coverage.out && cat *.cover.out | grep -v mode: | sort -r | \
awk '{if($1 != last) {print $0;last=$1}}' >> coverage.out
rm -rf *.cover.out