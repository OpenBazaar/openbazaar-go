package selfhosted

import (
	"crypto/sha256"
	"encoding/hex"
	peer "gx/ipfs/QmRBqJF7hb8ZSpRcMwUt8hNhydWcxGEhtk81HKq6oUwKvs/go-libp2p-peer"
	ma "gx/ipfs/QmYzDkkgAEmrcNzFCiYo6L1dTX4EAG1gZkbtdbd9trL4vd/go-multiaddr"
	"os"
	"path"

	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/ipfs/go-ipfs/commands"
)

type SelfHostedStorage struct {
	repoPath string
	context  commands.Context
}

func NewSelfHostedStorage(repoPath string, context commands.Context) *SelfHostedStorage {
	return &SelfHostedStorage{
		repoPath: repoPath,
		context:  context,
	}
}

func (s *SelfHostedStorage) Store(peerID peer.ID, ciphertext []byte) (ma.Multiaddr, error) {
	b := sha256.Sum256(ciphertext)
	hash := hex.EncodeToString(b[:])
	filePath := path.Join(s.repoPath, "outbox", hash)
	f, err := os.Create(filePath)
	defer f.Close()
	if err != nil {
		return nil, err
	}
	_, ferr := f.Write(ciphertext)
	if ferr != nil {
		return nil, ferr
	}
	addr, err := ipfs.AddFile(s.context, filePath)
	if err != nil {
		return nil, err
	}
	maAddr, err := ma.NewMultiaddr("/ipfs/" + addr + "/")
	if err != nil {
		return nil, err
	}
	return maAddr, nil
}
