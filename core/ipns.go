package core

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"gx/ipfs/QmTmqJGRQfuH8eKWD1FjThwPRipt1QhqJQNZ8MpzmfAAxo/go-ipfs-ds-help"
	"gx/ipfs/QmZ4Qi3GaRbjcx28Sme5eMH7RQjGkt8wHxt2a65oLaeFEV/gogo-protobuf/proto"
	"gx/ipfs/QmZoWKhxUmZ2seW4BzX6fJkNR8hh9PsGModr7q171yq2SS/go-libp2p-peer"
	"gx/ipfs/QmaPbCnUMBohSGo3KnxEa2bHqyJVVeEEcwtqJAYxerieBo/go-libp2p-crypto"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/OpenBazaar/openbazaar-go/ipfs"
	ipnspb "github.com/ipfs/go-ipfs/namesys/pb"
	npb "github.com/ipfs/go-ipfs/namesys/pb"
	ipfspath "github.com/ipfs/go-ipfs/path"
	ipnspath "github.com/ipfs/go-ipfs/path"
)

/*
These functions wrap the IPNS Resolve commands with code that will try fetching the record from an API
as a last ditch effort if it fails to find the record in the DHT. The API endpoint is loaded from the config file.
We need to take care to observe the Tor preference.
*/

// IPNSResolveThenCat - find the record in the DHT
func (n *OpenBazaarNode) IPNSResolveThenCat(ipnsPath ipfspath.Path, timeout time.Duration, usecache bool) ([]byte, error) {
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
	b, err := ipfs.Cat(n.IpfsNode, ipfspath.Join(p), timeout)
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
	val, err := ipfs.Resolve(n.IpfsNode, pid, timeout, usecache)
	if err != nil && n.IPNSBackupAPI != "" {
		dial := net.Dial
		if n.TorDialer != nil {
			dial = n.TorDialer.Dial
		}
		tbTransport := &http.Transport{Dial: dial}
		client := &http.Client{Transport: tbTransport, Timeout: time.Second * 5}
		resp, err := client.Get(n.IPNSBackupAPI + peerID)
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
			Pubkey           string `json:"pubkey"`
			SerializedRecord string `json:"serializedRecord"`
		}

		rec := new(KeyAndRecord)

		err = json.Unmarshal(b, rec)
		if err != nil {
			log.Error(err)
			return "", err
		}

		entry := new(ipnspb.IpnsEntry)
		entryBytes, err := hex.DecodeString(rec.SerializedRecord)
		if err != nil {
			log.Error(err)
			return "", err
		}
		err = proto.Unmarshal(entryBytes, entry)
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
			return "", fmt.Errorf("invalid key. Does not hash to %s", peerID)
		}

		// check sig with pk
		if ok, err := pubkey.Verify(ipnsEntryDataForSig(entry), entry.Signature); err != nil || !ok {
			log.Errorf("Signature on IPNS record from gateway validated to %t", ok)
			return "", fmt.Errorf("invalid value. Not signed by PrivateKey corresponding to %v", pubkey)
		}

		go func() {
			n.IpfsNode.Repo.Datastore().Put(dshelp.NewKeyFromBinary([]byte("/ipns/"+pid.Pretty())), entryBytes)
			n.IpfsNode.Repo.Datastore().Put(dshelp.NewKeyFromBinary([]byte(KeyCachePrefix+pid.Pretty())), pubkeyBytes)
		}()

		p, err := ipnspath.ParsePath(string(entry.Value))
		if err != nil {
			log.Error(err)
			return "", err
		}
		val = strings.TrimPrefix(p.String(), "/ipfs/")
		return val, nil
	}
	return val, err
}

func ipnsEntryDataForSig(e *npb.IpnsEntry) []byte {
	return bytes.Join([][]byte{
		e.Value,
		e.Validity,
		[]byte(fmt.Sprint(e.GetValidityType())),
	},
		[]byte{})
}
