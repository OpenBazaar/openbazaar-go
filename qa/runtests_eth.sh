#!/bin/bash
for SCRIPT in `ls | grep "eth_"`
do
   b=$(basename $SCRIPT)
   extension="${b##*.}"
   p="py"
   if [ $extension = $p ]
   then
      python3 $SCRIPT -b $1 -c "ETH" $2
      #if [[ $? -ne 0 ]]; then
        #kill -1 $$
      #fi
   fi
done
