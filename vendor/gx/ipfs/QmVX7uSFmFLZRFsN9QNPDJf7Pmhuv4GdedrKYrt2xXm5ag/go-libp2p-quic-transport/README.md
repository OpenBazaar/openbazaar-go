# go-quic-transport

[![Godoc Reference](https://img.shields.io/badge/godoc-reference-blue.svg?style=flat-square)](https://godoc.org/github.com/marten-seemann/libp2p-quic-transport)
[![Linux Build Status](https://img.shields.io/travis/marten-seemann/libp2p-quic-transport/master.svg?style=flat-square&label=linux+build)](https://travis-ci.org/marten-seemann/libp2p-quic-transport)
[![Code Coverage](https://img.shields.io/codecov/c/github/marten-seemann/libp2p-quic-transport/master.svg?style=flat-square)](https://codecov.io/gh/marten-seemann/libp2p-quic-transport/)

This is an implementation of the [libp2p transport](https://github.com/libp2p/go-libp2p-transport/blob/master/transport.go) and the [libp2p stream muxer](https://github.com/libp2p/go-stream-muxer) using QUIC.

## Known limitations

* currently only works with RSA host keys
