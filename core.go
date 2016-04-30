package main

import (
	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/commands"
	"github.com/OpenBazaar/openbazaar-go/net"
)

type OpenBazaarNode struct {
	// Context for issuing IPFS commands
	Context    commands.Context

	// IPFS node object
	IpfsNode   *core.IpfsNode

	// The roothash of the node directory inside the openbazaar repo.
	// This directory hash is published on IPNS at our peer ID making
	// the directory publically viewable on the network.
	RootHash   string

	// The path to the openbazaar repo in the file system.
	RepoPath   string

	// The OpenBazaar network service for direct communication between peers
	Service    *net.OpenBazaarService

	// Database for storing node specific data
	Datastore repo.Datastore

	// TODO: Offline Session Manager
	// TODO: Pointer Republisher
	// TODO: BitcoinWallet
}