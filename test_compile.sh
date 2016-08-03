#!/bin/bash

go test -coverprofile=api.cover.out ./api/*_test.go
go test -coverprofile=bitcoin.cover.out ./bitcoin/*_test.go
go test -coverprofile=core.cover.out ./core/*_test.go
go test -coverprofile=ipfs.cover.out ./ipfs/*_test.go
go test -coverprofile=net.cover.out ./net/*_test.go
go test -coverprofile=netservice.cover.out ./net/service/*_test.go
go test -coverprofile=pb.cover.out =./pb/*_test.go
go test -coverprofile=repo.cover.out =./repo/*_test.go
go test -coverprofile=repodb.cover.out ./repo/db/*_test.go
go test -coverprofile=storage.cover.out ./storage/*_test.go
go test -coverprofile=dropbox.cover.out ./storage/dropbox/*_test.go
go test -coverprofile=selfhosted.cover.out ./storage/selfhosted/*_test.go
echo "mode: set" > coverage.out && cat *.cover.out | grep -v mode: | sort -r | \
awk '{if($1 != last) {print $0;last=$1}}' >> coverage.out
rm -rf *.cover.out
