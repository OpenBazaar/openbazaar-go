# openbazaar-go
OpenBazaar Server Daemon in Go

[![Build Status](https://travis-ci.org/OpenBazaar/openbazaar-go.svg?branch=master)](https://travis-ci.org/OpenBazaar/openbazaar-go)
[![Coverage Status](https://coveralls.io/repos/github/OpenBazaar/openbazaar-go/badge.svg?branch=master)](https://coveralls.io/github/OpenBazaar/openbazaar-go?branch=master)
[![Go Report Card](https://goreportcard.com/badge/github.com/OpenBazaar/openbazaar-go)](https://goreportcard.com/report/github.com/OpenBazaar/openbazaar-go)

##### Install

Before installation set  `$GOPATH` to the `openbazaar-go` top-level directory. Also, OpenSSL is a dependency. On Mac OS X, use:

```
cd $GOPATH
wget https://www.openssl.org/source/openssl-1.0.1t.tar.gz
tar xzvf openssl-1.0.1t.tar.gz
cd openssl-1.0.1t
cp -rf include/openssl $GOPATH/src/github.com/OpenBazaar/openbazaar-go/vendor/github.com/xeodou/go-sqlcipher/
```

Finally install:

```
go get -u github.com/OpenBazaar/openbazaar-go
```

##### Run

```
cd $GOPATH/src/github.com/OpenBazaar/openbazaar-go
go run openbazaard.go start
```
