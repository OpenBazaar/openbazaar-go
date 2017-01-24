package selfhosted

import (
	"crypto/sha256"
	"encoding/hex"
	ma "gx/ipfs/QmUAQaWbKxGCUTuoQVvvicbQNZ9APF5pDGWyAZSe93AtKH/go-multiaddr"
	peer "gx/ipfs/QmfMmLGoKzCHDN7cGgk64PJr4iipzidDRME8HABSJqvmhC/go-libp2p-peer"
	"os"
	"path"

	"bytes"
	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/ipfs/go-ipfs/commands"
	"net/http"
	"net/url"
)

type SelfHostedStorage struct {
	repoPath          string
	context           commands.Context
	crossPostGateways []*url.URL
}

func NewSelfHostedStorage(repoPath string, context commands.Context, crossPostGateways []*url.URL) *SelfHostedStorage {
	return &SelfHostedStorage{
		repoPath:          repoPath,
		context:           context,
		crossPostGateways: crossPostGateways,
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
	for _, g := range s.crossPostGateways {
		http.Post(g.String()+"ipfs/", "application/x-www-form-urlencoded", bytes.NewReader(ciphertext))
	}
	maAddr, err := ma.NewMultiaddr("/ipfs/" + addr + "/")
	if err != nil {
		return nil, err
	}
	return maAddr, nil
}
