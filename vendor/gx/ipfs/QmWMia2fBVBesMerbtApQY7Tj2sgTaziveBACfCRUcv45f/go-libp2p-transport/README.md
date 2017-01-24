# go-libp2p-transport
[![](https://img.shields.io/badge/made%20by-Protocol%20Labs-blue.svg?style=flat-square)](http://ipn.io)
[![](https://img.shields.io/badge/freenode-%23ipfs-blue.svg?style=flat-square)](http://webchat.freenode.net/?channels=%23ipfs)
[![GoDoc](https://godoc.org/github.com/ipfs/go-libp2p-transport?status.svg)](https://godoc.org/github.com/ipfs/go-libp2p-transport)
[![Coverage Status](https://coveralls.io/repos/github/ipfs/go-libp2p-transport/badge.svg?branch=master)](https://coveralls.io/github/ipfs/go-libp2p-transport?branch=master)
[![Build Status](https://travis-ci.org/ipfs/go-libp2p-transport.svg?branch=master)](https://travis-ci.org/ipfs/go-libp2p-transport)

A common interface for network transports.

This is the 'base' layer for any transport that wants to be used by libp2p and ipfs. If you want to make 'ipfs work over X', the first thing you'll want to do is to implement the `Transport` interface for 'X'.

## Installation

```sh
> gx install --global
> gx-go rewrite
```

## Usage

```go
var t Transport

t = NewTCPTransport()

list, err := t.Listen(listener_maddr)
if err != nil {
	log.Fatal(err)
}

con, err := list.Accept()
if err != nil {
	log.Fatal(err)
}

fmt.Fprintln(con, "Hello World!")
```

## License
MIT
