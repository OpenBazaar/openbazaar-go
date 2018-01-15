# openbazaar-go
![banner](https://i.imgur.com/iOnXDXK.png)
OpenBazaar Server Daemon in Go

[![Build Status](https://travis-ci.org/OpenBazaar/openbazaar-go.svg?branch=master)](https://travis-ci.org/OpenBazaar/openbazaar-go)
[![Coverage Status](https://coveralls.io/repos/github/OpenBazaar/openbazaar-go/badge.svg?branch=master)](https://coveralls.io/github/OpenBazaar/openbazaar-go?branch=master)
[![Go Report Card](https://goreportcard.com/badge/github.com/OpenBazaar/openbazaar-go)](https://goreportcard.com/report/github.com/OpenBazaar/openbazaar-go)

This repository contains the OpenBazaar server daemon which handles the heavy lifting for the [OpenBazaar](https://openbazaar.org/) desktop application. The server combines several technologies: A modified [IPFS](https://ipfs.io) node, which itself combines ideas from Git, BitTorrent, and Kademlia. A lightweight SPV Bitcoin wallet for interacting with the Bitcoin network. And a JSON API which can be used by a user interface to control the node and browse the network. If you are looking for the OpenBazaar user interface code see [here](https://github.com/OpenBazaar/openbazaar-desktop).

## Table of Contents

- [Install](#install)
  - [Install prebuilt packages](#install-pre-built-packages)
  - [Build from Source](#build-from-source)
- [Dependency Management](#dependency-management)
  - [IPFS Dependency](#ipfs-dependency)
- [Updating](#updating)
- [Usage](#usage)
  - [Options](#options)
- [Documentation](#documentation)
- [Todo](#todo)
- [Contributing](#contributing)
- [License](#license)

## Install

A typical install of OpenBazaar contains a bundle of the server daemon and user interface. If this is what you are looking for you can find an installer at https://openbazaar.org/download.html. If you are looking to run the server daemon by itself or to contribute to developement see below for instructions.

### Install Pre-built Packages

The easiest way to run the server is to download a pre-built binary. You can find binaries of our latest release for each operating system [here](https://github.com/OpenBazaar/openbazaar-go/releases/).

### Build from Source

To build from source you will need to have Go installed and properly configured. Detailed instructions for installing Go and openbazaar-go on each operating system can be found in the [docs package](https://github.com/OpenBazaar/openbazaar-go/tree/master/docs).

## Dependency Management

We use [Godeps](https://github.com/tools/godep) with vendored third-party packages.

### IPFS Dependency

We are using a [fork](https://github.com/OpenBazaar/go-ipfs) of go-ipfs in the daemon. The primary changes include different protocol strings to segregate the OpenBazaar network from the main IPFS network and an increased TTL on certain types of DHT data. You can find the full diff in the readme of the forked repo. The fork is bundled in the vendor package and will be used automatically when you compile and run the server. Note that you will still see github.com/ipfs/go-ipfs import statements instead of github.com/OpenBazaar/go-ipfs despite the package being a fork. This is done to avoid a major refactor of import statements and make rebasing IPFS much more easy.

## Updating

You can either pull in remote changes as normal or run `go get -u github.com/OpenBazaar/openbazaar-go`

## Usage

You can run the server with `go run openbazaard.go start`

### Options
```
Usage:
  openbazaard [OPTIONS] start [start-OPTIONS]

The start command starts the OpenBazaar-Server

Application Options:
  -v, --version                   Print the version number and exit

Help Options:
  -h, --help                      Show this help message

[start command options]
      -p, --password=             the encryption password if the database is
                                  encrypted
      -t, --testnet               use the test network
      -r, --regtest               run in regression test mode
      -l, --loglevel=             set the logging level [debug, info, notice,
                                  warning, error, critical]
      -a, --allowip=              only allow API connections from these IPs
      -s, --stun                  use stun on µTP IPv4
      -d, --datadir=              specify the data directory to be used
      -c, --authcookie=           turn on API authentication and use this
                                  specific cookie
      -u, --useragent=            add a custom user-agent field
          --torpassword=          Set the tor control password. This will
                                  override the tor password in the config.
          --tor                   Automatically configure the daemon to run as
                                  a Tor hidden service and use Tor exclusively.
                                  Requires Tor to be running.
          --dualstack             Automatically configure the daemon to run as
                                  a Tor hidden service IN ADDITION to using the
                                  clear internet. Requires Tor to be running.
                                  WARNING: this mode is not private
          --disablewallet         disable the wallet functionality of the node
          --disableexchangerates  disable the exchange rate service to prevent
                                  api queries
          --storage=              set the outgoing message storage option
                                  [self-hosted, dropbox] default=self-hosted
```

## Documentation

When ready we will put up a full documentation of the OpenBazaar protocol and the JSON API at https://docs.openbazaar.org

## Contributing

Contributions are definitely welcome! Please read the contributing [guidelines](https://github.com/OpenBazaar/openbazaar-go/blob/master/CONTRIBUTE.md) before starting.

## License

MIT

