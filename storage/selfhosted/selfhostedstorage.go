package selfhosted

import (
	"crypto/sha256"
	"encoding/hex"
	ma "gx/ipfs/QmXY77cVe7rVRQXZZQRioukUM7aRW3BTcAgJe12MCtb3Ji/go-multiaddr"
	peer "gx/ipfs/QmXYjuNuxVzXKJCfWasQk1RqkhVLDM9jtUKhqc2WPQmFSB/go-libp2p-peer"
	"os"
	"path"

	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/ipfs/go-ipfs/commands"
	"gx/ipfs/QmNp85zy9RLrQ5oQD4hPyS39ezrrXpcaa7R4Y9kxdWQLLQ/go-cid"
)

type SelfHostedStorage struct {
	repoPath  string
	context   commands.Context
	pushNodes []peer.ID
	store     func(peerId string, ids []cid.Cid) error
}

func NewSelfHostedStorage(repoPath string, context commands.Context, pushNodes []peer.ID, store func(peerId string, ids []cid.Cid) error) *SelfHostedStorage {
	return &SelfHostedStorage{
		repoPath:  repoPath,
		context:   context,
		pushNodes: pushNodes,
		store:     store,
	}
}

func (s *SelfHostedStorage) Store(peerID peer.ID, ciphertext []byte) (ma.Multiaddr, error) {
	b := sha256.Sum256(ciphertext)
	hash := hex.EncodeToString(b[:])
	filePath := path.Join(s.repoPath, "outbox", hash)
	f, err := os.Create(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	_, ferr := f.Write(ciphertext)
	if ferr != nil {
		return nil, ferr
	}
	addr, err := ipfs.AddFile(s.context, filePath)
	if err != nil {
		return nil, err
	}
	id, err := cid.Decode(addr)
	if err != nil {
		return nil, err
	}
	for _, peer := range s.pushNodes {
		go s.store(peer.Pretty(), []cid.Cid{*id})
	}
	maAddr, err := ma.NewMultiaddr("/ipfs/" + addr + "/")
	if err != nil {
		return nil, err
	}
	return maAddr, nil
}
