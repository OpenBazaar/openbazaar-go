# go-libp2p-http

[![Build Status](https://travis-ci.org/hsanjuan/go-libp2p-http.svg?branch=master)](https://travis-ci.org/hsanjuan/go-libp2p-http)
[![codecov](https://codecov.io/gh/hsanjuan/go-libp2p-http/branch/master/graph/badge.svg)](https://codecov.io/gh/hsanjuan/go-libp2p-http)
[![standard-readme compliant](https://img.shields.io/badge/standard--readme-OK-green.svg)](https://github.com/RichardLitt/standard-readme)


> HTTP on top of LibP2P

Package `p2phttp` allows to serve HTTP endpoints and make HTTP requests through [LibP2P](https://github.com/libp2p/libp2p) using Go's standard "http" and "net" stack.

Instead of the regular "host:port" addressing, `p2phttp` uses a Peer ID and lets LibP2P take care of the routing, thus taking advantage of features like multi-routes,  NAT transversal and stream multiplexing over a single connection.

## Table of Contents

- [Install](#install)
- [Usage](#usage)
- [Contribute](#contribute)
- [License](#license)

## Install

This package uses [`gx`](https://github.com/whyrusleeping/gx-go) for dependencies and should be imported with `gx` on other projects:

```
$ gx import github.com/hsanjuan/go-libp2p-http
```

The code can be downloaded and tested with:

```
$ go get -u -d github.com/hsanjuan/go-libp2p-http
$ cd $GOPATH/src/github.com/hsanjuan/go-libp2p-http
$ make test
```

## Usage

Full documentation can be read at [Godoc](https://godoc.org/github.com/hsanjuan/go-libp2p-http). The important bits follow.

A simple http.Server on LibP2P works as:

```go
listener, _ := gostream.Listen(host1, p2phttp.P2PProtocol)
defer listener.Close()
go func() {
	http.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hi!"))
	})
	server := &http.Server{}
	server.Serve(listener)
}
```

The listener is provided by https://github.com/hsanjuan/go-libp2p-gostream .

A client just needs to be initialized with a custom LibP2P host-based transport to perform requests to such server:

```go
tr := &http.Transport{}
tr.RegisterProtocol("libp2p", p2phttp.NewTransport(clientHost))
client := &http.Client{Transport: tr}
res, err := client.Get("libp2p://Qmaoi4isbcTbFfohQyn28EiYM5CDWQx9QRCjDh3CTeiY7P/hello")
```

## Contribute

PRs accepted.

## License

MIT © Hector Sanjuan
