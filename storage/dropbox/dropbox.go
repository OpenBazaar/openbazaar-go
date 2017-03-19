package dropbox

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	ma "gx/ipfs/QmSWLfmj5frN9xVLMMN846dMDriy5wN5jeghUm7aTW3DAG/go-multiaddr"
	peer "gx/ipfs/QmWUswjn261LSyVxWAEpMVtPdy8zmKBJJfBpG3Qdpa8ZsE/go-libp2p-peer"
	mh "gx/ipfs/QmbZ6Cee2uHjG7hf19qLHppgKDRtaG4CVtMzdmK9VCVqLu/go-multihash"

	"github.com/dropbox/dropbox-sdk-go-unofficial"
	"github.com/dropbox/dropbox-sdk-go-unofficial/files"
	"github.com/dropbox/dropbox-sdk-go-unofficial/sharing"
)

type DropBoxStorage struct {
	apiToken string
}

func NewDropBoxStorage(apiToken string) (*DropBoxStorage, error) {
	api := dropbox.Client(apiToken, dropbox.Options{Verbose: true})
	if _, err := api.GetCurrentAccount(); err != nil {
		return nil, err
	}
	return &DropBoxStorage{
		apiToken: apiToken,
	}, nil
}

func (s *DropBoxStorage) Store(peerID peer.ID, ciphertext []byte) (ma.Multiaddr, error) {
	api := dropbox.Client(s.apiToken, dropbox.Options{Verbose: true})
	hash := sha256.Sum256(ciphertext)
	hex := hex.EncodeToString(hash[:])

	// Upload ciphertext
	uploadArg := files.NewCommitInfo("/" + hex)
	r := bytes.NewReader(ciphertext)
	_, err := api.Upload(uploadArg, r)
	if err != nil {
		return nil, err
	}

	// Set public sharing
	sharingArg := sharing.NewCreateSharedLinkArg("/" + hex)
	res, err := api.CreateSharedLink(sharingArg)
	if err != nil {
		return nil, err
	}

	// Create encoded multiaddr
	url := res.Url[:len(res.Url)-1] + "1"
	b, err := mh.Encode([]byte(url), mh.SHA1)
	if err != nil {
		return nil, err
	}
	m, err := mh.Cast(b)
	if err != nil {
		return nil, err
	}

	addr, err := ma.NewMultiaddr("/ipfs/" + m.B58String() + "/https/")
	if err != nil {
		return nil, err
	}
	return addr, nil
}
