base58
==========

[![Build Status](http://img.shields.io/travis/ltcsuite/ltcutil.svg)]
(https://travis-ci.org/ltcsuite/ltcutil) [![ISC License]
(http://img.shields.io/badge/license-ISC-blue.svg)](http://copyfree.org)
[![GoDoc](https://godoc.org/github.com/ltcsuite/ltcutil/base58?status.png)]
(http://godoc.org/github.com/ltcsuite/ltcutil/base58)

Package base58 provides an API for encoding and decoding to and from the
modified base58 encoding.  It also provides an API to do Base58Check encoding,
as described [here](https://en.litecoin.it/wiki/Base58Check_encoding).

A comprehensive suite of tests is provided to ensure proper functionality.

## Installation and Updating

```bash
$ go get -u github.com/ltcsuite/ltcutil/base58
```

## Examples

* [Decode Example]
  (http://godoc.org/github.com/ltcsuite/ltcutil/base58#example-Decode)  
  Demonstrates how to decode modified base58 encoded data.
* [Encode Example]
  (http://godoc.org/github.com/ltcsuite/ltcutil/base58#example-Encode)  
  Demonstrates how to encode data using the modified base58 encoding scheme.
* [CheckDecode Example]
  (http://godoc.org/github.com/ltcsuite/ltcutil/base58#example-CheckDecode)  
  Demonstrates how to decode Base58Check encoded data.
* [CheckEncode Example]
  (http://godoc.org/github.com/ltcsuite/ltcutil/base58#example-CheckEncode)  
  Demonstrates how to encode data using the Base58Check encoding scheme.

## License

Package base58 is licensed under the [copyfree](http://copyfree.org) ISC
License.
