ltcutil
=======

[![Build Status](http://img.shields.io/travis/ltcsuite/ltcutil.svg)](https://travis-ci.org/ltcsuite/ltcutil) 
[![Coverage Status](http://img.shields.io/coveralls/ltcsuite/ltcutil.svg)](https://coveralls.io/r/ltcsuite/ltcutil?branch=master) 
[![ISC License](http://img.shields.io/badge/license-ISC-blue.svg)](http://copyfree.org)
[![GoDoc](http://img.shields.io/badge/godoc-reference-blue.svg)](http://godoc.org/github.com/ltcsuite/ltcutil)

Package ltcutil provides litecoin-specific convenience functions and types.
A comprehensive suite of tests is provided to ensure proper functionality.  See
`test_coverage.txt` for the gocov coverage report.  Alternatively, if you are
running a POSIX OS, you can run the `cov_report.sh` script for a real-time
report.

This package was developed for ltcd, an alternative full-node implementation of
litecoin based on btcd, which is under active development by Conformal.
Although it was primarily written for ltcd, this package has intentionally been
designed so it can be used as a standalone package for any projects needing the
functionality provided.

## Installation and Updating

```bash
$ go get -u github.com/ltcsuite/ltcutil
```

## License

Package ltcutil is licensed under the [copyfree](http://copyfree.org) ISC
License.
