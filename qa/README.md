# OpenBazaar Integration Test Suite

This package allows us to spin up any number of OpenBazaar nodes and connect them all in a private network for use in testing. Python test scripts in this package run automatically on all pull requests and commits.

To run this test suite:
```
./runtests.sh /path/to/openbazaar-go-binary /path/to/bitcoind-binary
```