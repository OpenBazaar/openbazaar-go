package core

import (
	"path"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/commands"
	"github.com/OpenBazaar/openbazaar-go/net"
	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/OpenBazaar/openbazaar-go/bitcoin"
)

var Node *OpenBazaarNode

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
	Service    net.NetworkService

	// Database for storing node specific data
	Datastore  repo.Datastore

	// Websocket channel used for pushing data to the UI.
	Broadcast  chan []byte

	// Bitcoin wallet implementation
	Wallet 	   bitcoin.BitcoinWallet

	// TODO: Libsignal Client
	// TODO: Pointer Republisher
}

// Unpin the current node repo, re-add it, then publish to ipns
func (n *OpenBazaarNode) SeedNode() error {
	if err := ipfs.UnPinDir(n.Context, n.RootHash); err != nil {
		return err
	}
	hash, aerr := ipfs.AddDirectory(n.Context, path.Join(n.RepoPath, "root"))
	if aerr != nil {
		return aerr
	}
	_, perr := ipfs.Publish(n.Context, hash)
	if perr != nil {
		return perr
	}
	n.RootHash = hash
	return nil
}