package core

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/ipfs/go-ipfs/namesys"
	"gx/ipfs/QmPvyPwuCgJ7pDmrKDxRtsScJgBaM5h4EpRL2qQJsmXf4n/go-libp2p-crypto"
	"gx/ipfs/QmTRhk7cgjUf2gfQ3p2M9KPECNZEW9XUrmHcFCgog4cPgB/go-libp2p-peer"
	"gx/ipfs/QmaRb5yNXKonhbkpNxNawoydk4N6es6b4fPj19sjEKsh5D/go-datastore"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/OpenBazaar/openbazaar-go/ipfs"
	ipath "gx/ipfs/QmT3rzed1ppXefourpmoZ7tyVQfsGPQZ1pHDngLmCvXxd3/go-path"
	ipnspb "gx/ipfs/QmaRFtZhVAwXBk4Z3zEsvjScH9fjsDZmhXfa1Gm8eMb9cg/go-ipns/pb"
)

/*
These functions wrap the IPNS Resolve commands with code that will try fetching the record from an API
as a last ditch effort if it fails to find the record in the DHT. The API endpoint is loaded from the config file.
We need to take care to observe the Tor preference.
*/

// IPNSResolveThenCat - find the record in the DHT
func (n *OpenBazaarNode) IPNSResolveThenCat(ipnsPath ipath.Path, timeout time.Duration, usecache bool) ([]byte, error) {
	var ret []byte
	hash, err := n.IPNSResolve(ipnsPath.Segments()[0], timeout, usecache)
	if err != nil {
		return ret, err
	}
	p := make([]string, len(ipnsPath.Segments()))
	p[0] = hash
	for i := 0; i < len(ipnsPath.Segments())-1; i++ {
		p[i+1] = ipnsPath.Segments()[i+1]
	}
	b, err := ipfs.Cat(n.IpfsNode, ipath.Join(p), timeout)
	if err != nil {
		return ret, err
	}
	return b, nil
}

// IPNSResolve - try fetching the record from an API
func (n *OpenBazaarNode) IPNSResolve(peerID string, timeout time.Duration, usecache bool) (string, error) {
	pid, err := peer.IDB58Decode(peerID)
	if err != nil {
		return "", err
	}
	val, err := ipfs.Resolve(n.IpfsNode, pid, timeout, n.IPNSQuorumSize, usecache)
	if err != nil && n.IPNSBackupAPI != "" {
		dial := net.Dial
		if n.TorDialer != nil {
			dial = n.TorDialer.Dial
		}
		tbTransport := &http.Transport{Dial: dial}
		client := &http.Client{Transport: tbTransport, Timeout: time.Second * 5}
		resp, err := client.Get(ipnsAPIPathTransform(n.IPNSBackupAPI, peerID))
		if err != nil {
			log.Error(err)
			return "", err
		}
		if resp.StatusCode != http.StatusOK {
			return "", errors.New("IPNS record not found in network")
		}

		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Error(err)
			return "", err
		}

		type KeyAndRecord struct {
			Pubkey string `json:"pubkey"`
			Record string `json:"record"`
		}

		rec := new(KeyAndRecord)

		err = json.Unmarshal(b, rec)
		if err != nil {
			log.Error(err)
			return "", err
		}

		pubkeyBytes, err := hex.DecodeString(rec.Pubkey)
		if err != nil {
			log.Error(err)
			return "", err
		}

		pubkey, err := crypto.UnmarshalPublicKey(pubkeyBytes)
		if err != nil {
			log.Error(err)
			return "", err
		}
		id, err := peer.IDB58Decode(peerID)
		if err != nil {
			log.Error(err)
			return "", err
		}
		if !id.MatchesPublicKey(pubkey) {
			log.Error(err)
			return "", fmt.Errorf("Invalid key. Does not hash to %s", peerID)
		}

		p, err := ipath.ParsePath(string(rec.Record))
		if err != nil {
			log.Error(err)
			return "", err
		}

		go func() {
			n.IpfsNode.Repo.Datastore().Put(namesys.IpnsDsKey(id), []byte(p.String()))
			n.IpfsNode.Repo.Datastore().Put(datastore.NewKey(KeyCachePrefix+pid.Pretty()), pubkeyBytes)
		}()

		val = strings.TrimPrefix(p.String(), "/ipfs/")
		return val, nil
	}
	return val, err
}

func ipnsEntryDataForSig(e *ipnspb.IpnsEntry) []byte {
	return bytes.Join([][]byte{
		e.Value,
		e.Validity,
		[]byte(fmt.Sprint(e.GetValidityType())),
	},
		[]byte{})
}

func ipnsAPIPathTransform(url, peerID string) string {
	if !strings.HasSuffix(url, "/") {
		url = url + "/"
	}
	return url + "ob/ipns/" + peerID
}
