#!/bin/bash
# $1 is the openbazaar binary path
# $2 is the bitcoind path
# $3 will filter to match against script name
for SCRIPT in `ls | grep -v "eth_"`
do
   b=$(basename $SCRIPT)
   extension="${b##*.}"
   p="py"
   if [[ $extension = $p ]]; then
     if [[ -z $3 ]]; then
       echo "python3 $SCRIPT -b $1 -d $2"
       python3 $SCRIPT -b $1 -d $2
       #if [[ $? -ne 0 ]]; then
         #kill -1 $$
       #fi
     else
       # filter only the scripts of interest
       if [[ $SCRIPT == *"$3"* ]]; then
         echo "python3 $SCRIPT -b $1 -d $2"
         python3 $SCRIPT -b $1 -d $2
         #if [[ $? -ne 0 ]]; then
           #kill -1 $$
         #fi
       fi
     fi
   fi
done
