#!/bin/bash

echo 'Building openbazaar...'
go build -o /opt/openbazaard .

# Run QA suite
cd qa
./runtests.sh /opt/openbazaard /opt/bitcoin-0.16.3/bin/bitcoind
