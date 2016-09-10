#!/bin/bash

set -e
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

wget https://bitcoin.org/bin/bitcoin-core-0.13.0/bitcoin-0.13.0-x86_64-linux-gnu.tar.gz
tar -xvzf bitcoin-0.13.0-x86_64-linux-gnu.tar.gz

cd qa
for SCRIPT in *
do
   b=$(basename $SCRIPT)
   extension="${b##*.}"
   p="py"
   if [ $extension = $p ]
   then
      python3 $SCRIPT -b $GOPATH/bin/openbazaar-go -d $GOPATH/src/openbazaar-go/bitcoin-0.13.0/bin/bitcoind
   fi
done