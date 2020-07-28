# go-address
[![](https://img.shields.io/badge/made%20by-Protocol%20Labs-blue.svg?style=flat-square)](http://ipn.io)
[![CircleCI](https://circleci.com/gh/filecoin-project/go-address.svg?style=svg)](https://circleci.com/gh/filecoin-project/go-address)
[![codecov](https://codecov.io/gh/filecoin-project/go-address/branch/master/graph/badge.svg)](https://codecov.io/gh/filecoin-project/go-address)

The filecoin address type, used for identifying actors on the filecoin network, in various formats.

## Install

Install this library with `go mod`

## Usage

Addresses support various types of encoding formats and have constructors
for each format

```golang
// address from ID
idAddress := NewIDAddress(id)
// address from a secp pub key
secp256k1Address := NewSecp256k1Address(pubkey)
// address from data for actor protocol
actorAddress := NewActorAddress(data) 
// address from the BLS pubkey
blsAddress := NewBLSAddress(pubkey)
```

Serialization

```golang
var outBuf io.writer
err := address.MarshalCBOR(outbuf)
var inBuf io.reader
err := address.UnmarshalCBOR(inbuf)
```

## Project-level documentation
The filecoin-project has a [community repo](https://github.com/filecoin-project/community) that documents in more detail our policies and guidelines, such as discussion forums and chat rooms and  [Code of Conduct](https://github.com/filecoin-project/community/blob/master/CODE_OF_CONDUCT.md).

## License
This repository is dual-licensed under Apache 2.0 and MIT terms.

Copyright 2019. Protocol Labs, Inc.
