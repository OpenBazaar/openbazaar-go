package selfhosted

import (
	"crypto/sha256"
	"encoding/hex"
	"log"

	ma "gx/ipfs/QmTZBfrPJmjWsCvHEtX5FE6KimVJhsJg5sBbqEFYf4UZtL/go-multiaddr"
	"gx/ipfs/QmTbxNB1NwDesLmKTscr4udL2tVP7MaxvXnD1D9yX7g3PN/go-cid"
	peer "gx/ipfs/QmYVXrKrKHDC9FobgmcmshCDyWwdrfwfanNQN4oxJ9Fk3h/go-libp2p-peer"

	"os"
	"path"

	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/ipfs/go-ipfs/core"
)

type SelfHostedStorage struct {
	repoPath  string
	ipfsNode  *core.IpfsNode
	pushNodes []peer.ID
	store     func(peerId string, ids []cid.Cid) error
}

func NewSelfHostedStorage(repoPath string, n *core.IpfsNode, pushNodes []peer.ID, store func(peerId string, ids []cid.Cid) error) *SelfHostedStorage {
	return &SelfHostedStorage{
		repoPath:  repoPath,
		ipfsNode:  n,
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
	addr, err := ipfs.AddFile(s.ipfsNode, filePath)
	if err != nil {
		return nil, err
	}
	id, err := cid.Decode(addr)
	if err != nil {
		return nil, err
	}

	for _, peer := range s.pushNodes {
		go func(peerID string, cid []cid.Cid) {
			err := s.store(peerID, cid)
			if err != nil {
				log.Println(err)
			}
		}(peer.Pretty(), []cid.Cid{id})
	}
	maAddr, err := ma.NewMultiaddr("/ipfs/" + addr + "/")
	if err != nil {
		return nil, err
	}
	return maAddr, nil
}
