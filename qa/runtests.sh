#!/bin/bash
for SCRIPT in *
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
