#!/bin/bash
set -e
go test -i
go test -coverprofile=coverage.out ./...


# Maybe activate someday, but right now this takes a long time and has a hard time
# running without some kind of failure due to timeout lengths.
#wget https://bitcoin.org/bin/bitcoin-core-0.16.0/bitcoin-0.16.0-x86_64-linux-gnu.tar.gz
#tar -xvzf bitcoin-0.16.0-x86_64-linux-gnu.tar.gz -C /tmp

#cd qa
#chmod a+x runtests.sh
#./runtests.sh $GOPATH/bin/openbazaar-go /tmp/bitcoin-0.16.0/bin/bitcoind
