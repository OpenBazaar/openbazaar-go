#!/bin/bash

mkdir -p /var/openbazaar
/opt/dummy --datadir=/var/openbazaar
cat /var/openbazaar/root/listings/index.json
/opt/openbazaard start -t -d /var/openbazaar -l debug
