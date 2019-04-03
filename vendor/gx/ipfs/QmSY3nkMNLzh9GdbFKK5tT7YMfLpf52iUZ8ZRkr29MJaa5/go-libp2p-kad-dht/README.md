# go-libp2p-kad-dht

This is the OpenBazaar fork of the libp2p DHT. It maintains a minimal patch set on top of the base library to support the
OpenBazaar messaging system. Specifically some TTLs are increased to support longer data caching of IPNS records and certain
types of providers. 

All the lines in the code that have been changed are commented and the comments are prefixed with `// OpenBazaar:` so you can
easily search for them and get the context of the change.