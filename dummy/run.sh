#!/bin/bash

mkdir -p /var/openbazaar
/opt/dummy --datadir=/var/openbazaar
/opt/openbazaard start -t -d /var/openbazaar -l debug
