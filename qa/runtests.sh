#!/bin/bash
# $1 is the openbazaar binary path
# $2 is the bitcoind path
# $3 will filter to match against script name
for SCRIPT in `ls | grep -v "eth_"`
do
   b=$(basename $SCRIPT)
   extension="${b##*.}"
   p="py"
   if [[ $extension = $p ]]
   then
      python3 $SCRIPT -b $1 -d $2 $3
      ret=$?
      if [[ $ret -ne 0 ]]; then
        kill -1 $$
      fi
   fi
done
