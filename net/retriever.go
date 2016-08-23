package net

import (
	peer "gx/ipfs/QmRBqJF7hb8ZSpRcMwUt8hNhydWcxGEhtk81HKq6oUwKvs/go-libp2p-peer"
	multihash "gx/ipfs/QmYf7ng2hG5XBtJA3tN34DQ2GUN5HNksEw1rLDkmr6vGku/go-multihash"
	ma "gx/ipfs/QmYzDkkgAEmrcNzFCiYo6L1dTX4EAG1gZkbtdbd9trL4vd/go-multiaddr"
	"io"
	"io/ioutil"
	"net/http"
	"time"

	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/OpenBazaar/openbazaar-go/net/service"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/golang/protobuf/proto"
	"github.com/ipfs/go-ipfs/commands"
	"github.com/ipfs/go-ipfs/core"
	routing "github.com/ipfs/go-ipfs/routing/dht"
	"golang.org/x/crypto/hkdf"
	"golang.org/x/net/context"
)

var salt = []byte("salt")

type MessageRetriever struct {
	db        repo.Datastore
	node      *core.IpfsNode
	ctx       commands.Context
	service   *service.OpenBazaarService
	prefixLen int
	sendAck   func(peerId string, pointerID peer.ID) error
}

func NewMessageRetriever(db repo.Datastore, ctx commands.Context, node *core.IpfsNode, service *service.OpenBazaarService, prefixLen int, sendAck func(peerId string, pointerID peer.ID) error) *MessageRetriever {
	return &MessageRetriever{
		db:        db,
		node:      node,
		ctx:       ctx,
		service:   service,
		prefixLen: prefixLen,
		sendAck:   sendAck,
	}
}

func (m *MessageRetriever) Run() {
	tick := time.NewTicker(time.Hour)
	defer tick.Stop()
	go m.fetchPointers()
	for {
		select {
		case <-tick.C:
			go m.fetchPointers()
		}
	}
}

func (m *MessageRetriever) fetchPointers() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mh, _ := multihash.FromB58String(m.node.Identity.Pretty())

	peerOut := ipfs.FindPointersAsync(m.node.Routing.(*routing.IpfsDHT), ctx, mh, m.prefixLen)
	for p := range peerOut {
		if len(p.Addrs) > 0 && !m.db.OfflineMessages().Has(p.Addrs[0].String()) {
			// ipfs
			if len(p.Addrs[0].Protocols()) == 1 && p.Addrs[0].Protocols()[0].Code == 421 {
				go m.fetchIPFS(m.ctx, p.ID, p.Addrs[0])
			}
			// https
			if len(p.Addrs[0].Protocols()) == 2 && p.Addrs[0].Protocols()[0].Code == 421 && p.Addrs[0].Protocols()[1].Code == 443 {
				enc, err := p.Addrs[0].ValueForProtocol(421)
				if err != nil {
					continue
				}
				mh, err := multihash.FromB58String(enc)
				if err != nil {
					continue
				}
				d, err := multihash.Decode(mh)
				if err != nil {
					continue
				}
				go m.fetchHTTPS(p.ID, string(d.Digest))
			}
			m.db.OfflineMessages().Put(p.Addrs[0].String())
		}
	}
}

func (m *MessageRetriever) fetchIPFS(ctx commands.Context, pid peer.ID, addr ma.Multiaddr) {
	ciphertext, err := ipfs.Cat(ctx, addr.String())
	if err != nil {
		return
	}
	m.attemptDecrypt(ciphertext, pid)
}

func (m *MessageRetriever) fetchHTTPS(pid peer.ID, url string) {
	resp, err := http.Get(url)
	if err != nil {
		return
	}
	ciphertext, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}
	m.attemptDecrypt(ciphertext, pid)
}

func (m *MessageRetriever) attemptDecrypt(ciphertext []byte, pid peer.ID) {
	if len(ciphertext) < 548 {
		return
	}
	symmetricKey, err := m.node.PrivateKey.Decrypt(ciphertext[4:516])
	if err != nil {
		return
	}
	hash := sha256.New

	hkdf := hkdf.New(hash, symmetricKey, salt, nil)

	aesKey := make([]byte, 32)
	_, err = io.ReadFull(hkdf, aesKey)
	if err != nil {
		return nil, err
	}
	macKey := make([]byte, 32)
	_, err = io.ReadFull(hkdf, macKey)
	if err != nil {
		return nil, err
	}

	mac := hmac.New(sha256.New, macKey)
	mac.Write(ciphertext[516 : len(ciphertext)-32])
	messageMac := mac.Sum(nil)
	if !hmac.Equal(messageMac, ciphertext[len(ciphertext)-32:]) {
		return
	}

	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return
	}
	ciphertext = ciphertext[516 : len(ciphertext)-32]
	if len(ciphertext) < aes.BlockSize {
		return
	}
	iv := ciphertext[:aes.BlockSize]
	ciphertext = ciphertext[aes.BlockSize:]

	stream := cipher.NewCFBDecrypter(block, iv)

	// XORKeyStream can work in-place if the two arguments are the same.
	stream.XORKeyStream(ciphertext, ciphertext)
	plaintext := ciphertext

	if err == nil {
		env := pb.Envelope{}
		err := proto.Unmarshal(plaintext, &env)
		if err != nil {
			return
		}
		id, err := peer.IDB58Decode(env.PeerID)
		if err != nil {
			return
		}
		// get handler for this msg type.
		handler := m.service.HandlerForMsgType(env.Message.MessageType)
		if handler == nil {
			log.Debug("Got back nil handler from handlerForMsgType")
			return
		}

		// dispatch handler.
		_, err = handler(id, env.Message)
		if err != nil {
			log.Debugf("handle message error: %s", err)
			return
		}

		if env.Message.MessageType != pb.Message_OFFLINE_ACK {
			m.sendAck(id.Pretty(), pid)
		}
	}
}
