for SCRIPT in *
do
   b=$(basename $SCRIPT)
   r="README.md"
   if [ $b != $r ]
   then 
   	python3 $SCRIPT -b $GOPATH/bin/openbazaar-go
   fi
   
done
