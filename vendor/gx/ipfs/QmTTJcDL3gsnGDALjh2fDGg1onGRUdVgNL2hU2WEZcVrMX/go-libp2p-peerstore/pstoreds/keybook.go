package pstoreds

import (
	"context"
	"errors"

	base32 "gx/ipfs/QmfVj3x4D6Jkq9SEoi5n2NmoUomLwoeiwnYz2KQa15wRw6/base32"

	ds "gx/ipfs/QmaRb5yNXKonhbkpNxNawoydk4N6es6b4fPj19sjEKsh5D/go-datastore"
	query "gx/ipfs/QmaRb5yNXKonhbkpNxNawoydk4N6es6b4fPj19sjEKsh5D/go-datastore/query"

	ic "gx/ipfs/QmPvyPwuCgJ7pDmrKDxRtsScJgBaM5h4EpRL2qQJsmXf4n/go-libp2p-crypto"
	peer "gx/ipfs/QmTRhk7cgjUf2gfQ3p2M9KPECNZEW9XUrmHcFCgog4cPgB/go-libp2p-peer"
	pstore "gx/ipfs/QmTTJcDL3gsnGDALjh2fDGg1onGRUdVgNL2hU2WEZcVrMX/go-libp2p-peerstore"
)

// Public and private keys are stored under the following db key pattern:
// /peers/keys/<b32 peer id no padding>/{pub, priv}
var (
	kbBase     = ds.NewKey("/peers/keys")
	pubSuffix  = ds.NewKey("/pub")
	privSuffix = ds.NewKey("/priv")
)

type dsKeyBook struct {
	ds ds.TxnDatastore
}

var _ pstore.KeyBook = (*dsKeyBook)(nil)

func NewKeyBook(_ context.Context, store ds.TxnDatastore, _ Options) (pstore.KeyBook, error) {
	return &dsKeyBook{store}, nil
}

func (kb *dsKeyBook) PubKey(p peer.ID) ic.PubKey {
	key := kbBase.ChildString(base32.RawStdEncoding.EncodeToString([]byte(p))).Child(pubSuffix)

	var pk ic.PubKey
	if value, err := kb.ds.Get(key); err == nil {
		pk, err = ic.UnmarshalPublicKey(value)
		if err != nil {
			log.Errorf("error when unmarshalling pubkey from datastore for peer %s: %s\n", p.Pretty(), err)
		}
	} else if err == ds.ErrNotFound {
		pk, err = p.ExtractPublicKey()
		switch err {
		case nil:
		case peer.ErrNoPublicKey:
			return nil
		default:
			log.Errorf("error when extracting pubkey from peer ID for peer %s: %s\n", p.Pretty(), err)
			return nil
		}
		pkb, err := pk.Bytes()
		if err != nil {
			log.Errorf("error when turning extracted pubkey into bytes for peer %s: %s\n", p.Pretty(), err)
			return nil
		}
		err = kb.ds.Put(key, pkb)
		if err != nil {
			log.Errorf("error when adding extracted pubkey to peerstore for peer %s: %s\n", p.Pretty(), err)
			return nil
		}
	} else {
		log.Errorf("error when fetching pubkey from datastore for peer %s: %s\n", p.Pretty(), err)
	}

	return pk
}

func (kb *dsKeyBook) AddPubKey(p peer.ID, pk ic.PubKey) error {
	// check it's correct.
	if !p.MatchesPublicKey(pk) {
		return errors.New("peer ID does not match public key")
	}

	key := kbBase.ChildString(base32.RawStdEncoding.EncodeToString([]byte(p))).Child(pubSuffix)
	val, err := pk.Bytes()
	if err != nil {
		log.Errorf("error while converting pubkey byte string for peer %s: %s\n", p.Pretty(), err)
		return err
	}
	err = kb.ds.Put(key, val)
	if err != nil {
		log.Errorf("error while updating pubkey in datastore for peer %s: %s\n", p.Pretty(), err)
	}
	return err
}

func (kb *dsKeyBook) PrivKey(p peer.ID) ic.PrivKey {
	key := kbBase.ChildString(base32.RawStdEncoding.EncodeToString([]byte(p))).Child(privSuffix)
	value, err := kb.ds.Get(key)
	if err != nil {
		log.Errorf("error while fetching privkey from datastore for peer %s: %s\n", p.Pretty(), err)
		return nil
	}
	sk, err := ic.UnmarshalPrivateKey(value)
	if err != nil {
		return nil
	}
	return sk
}

func (kb *dsKeyBook) AddPrivKey(p peer.ID, sk ic.PrivKey) error {
	if sk == nil {
		return errors.New("private key is nil")
	}
	// check it's correct.
	if !p.MatchesPrivateKey(sk) {
		return errors.New("peer ID does not match private key")
	}

	key := kbBase.ChildString(base32.RawStdEncoding.EncodeToString([]byte(p))).Child(privSuffix)
	val, err := sk.Bytes()
	if err != nil {
		log.Errorf("error while converting privkey byte string for peer %s: %s\n", p.Pretty(), err)
		return err
	}
	err = kb.ds.Put(key, val)
	if err != nil {
		log.Errorf("error while updating privkey in datastore for peer %s: %s\n", p.Pretty(), err)
	}
	return err
}

func (kb *dsKeyBook) PeersWithKeys() peer.IDSlice {
	ids, err := uniquePeerIds(kb.ds, kbBase, func(result query.Result) string {
		return ds.RawKey(result.Key).Parent().Name()
	})
	if err != nil {
		log.Errorf("error while retrieving peers with keys: %v", err)
	}
	return ids
}
