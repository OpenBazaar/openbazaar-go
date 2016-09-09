# OpenBazaar Integration Test Suite

This package allows us to spin up any number of OpenBazaar nodes and connect them all in a private network for use in testing. Python test scripts in this package run automatically on all pull requests and commits.

TODO: We need to make a config option for the spvwallet to connect to a single trusted peer. Once that functionality is available this test suite can then connect to bitcoind in regtest mode and make mock bitcoin transactions and purchases.
