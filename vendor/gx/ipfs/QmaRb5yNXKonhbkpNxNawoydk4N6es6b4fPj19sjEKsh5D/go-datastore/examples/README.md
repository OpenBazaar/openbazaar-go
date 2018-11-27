Example datastore implementations
---------------------------------

This directory contains simple implementation of the datastore interface

**THE CODE IN THIS DIRECTORY IS NOT SAFE TO USE IN ANY APPLICATION!**

If you are looking for a more complete persistent implementation of the
go-datastore interface, there are several implementations you can choose from:
* https://github.com/ipfs/go-ds-flatfs - Filesystem backed implementation,
  good for big blobs, though may be missing few features (some iteration
  settings don't work).
* https://github.com/ipfs/go-ds-leveldb - Datastore implementation backed
  by LevelDB database. Geed for small-size values and many keys.
* https://github.com/ipfs/go-ds-badger - A fast datastore implementation
  backed by BadgerDB. Good for most kinds of data.
