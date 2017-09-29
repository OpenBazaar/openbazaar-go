package selfhosted

import (
	"crypto/sha256"
	"encoding/hex"
	ma "gx/ipfs/QmXY77cVe7rVRQXZZQRioukUM7aRW3BTcAgJe12MCtb3Ji/go-multiaddr"
	peer "gx/ipfs/QmXYjuNuxVzXKJCfWasQk1RqkhVLDM9jtUKhqc2WPQmFSB/go-libp2p-peer"
	"os"
	"path"

	"bytes"
	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/ipfs/go-ipfs/commands"
	"golang.org/x/net/proxy"
	"net"
	"net/http"
	"net/url"
	"time"
)

type SelfHostedStorage struct {
	repoPath          string
	context           commands.Context
	crossPostGateways []*url.URL
	httpClient        *http.Client
}

func NewSelfHostedStorage(repoPath string, context commands.Context, crossPostGateways []*url.URL, dialer proxy.Dialer) *SelfHostedStorage {
	dial := net.Dial
	if dialer != nil {
		dial = dialer.Dial
	}
	tbTransport := &http.Transport{Dial: dial}
	client := &http.Client{Transport: tbTransport, Timeout: time.Minute}
	return &SelfHostedStorage{
		repoPath:          repoPath,
		context:           context,
		crossPostGateways: crossPostGateways,
		httpClient:        client,
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
		s.httpClient.Post(g.String()+"ipfs/", "application/x-www-form-urlencoded", bytes.NewReader(ciphertext))
	}
	maAddr, err := ma.NewMultiaddr("/ipfs/" + addr + "/")
	if err != nil {
		return nil, err
	}
	return maAddr, nil
}
