package api

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"gx/ipfs/QmTmqJGRQfuH8eKWD1FjThwPRipt1QhqJQNZ8MpzmfAAxo/go-ipfs-ds-help"
	"net/http"
	"os"
	"path"
	"time"

	ipnspb "github.com/ipfs/go-ipfs/namesys/pb"

	"github.com/OpenBazaar/openbazaar-go/core"
	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/golang/protobuf/proto"
	peer "github.com/libp2p/go-libp2p-peer"
)

func (i *jsonAPIHandler) GETStatus(w http.ResponseWriter, r *http.Request) {
	_, peerID := path.Split(r.URL.Path)
	status, err := i.node.GetPeerStatus(peerID)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	SanitizedResponse(w, fmt.Sprintf(`{"status": "%s"}`, status))
}

func (i *jsonAPIHandler) GETPeers(w http.ResponseWriter, r *http.Request) {
	peers := ipfs.ConnectedPeers(i.node.IpfsNode)
	var ret []string
	for _, p := range peers {
		ret = append(ret, p.Pretty())
	}
	peerJSON, err := json.MarshalIndent(ret, "", "    ")
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	SanitizedResponse(w, string(peerJSON))
}

func (i *jsonAPIHandler) GETClosestPeers(w http.ResponseWriter, r *http.Request) {
	_, peerID := path.Split(r.URL.Path)
	var peerIDs []string
	peers, err := ipfs.Query(i.node.IpfsNode, peerID)
	if err == nil {
		for _, p := range peers {
			peerIDs = append(peerIDs, p.Pretty())
		}
	}
	ret, _ := json.MarshalIndent(peerIDs, "", "    ")
	if isNullJSON(ret) {
		ret = []byte("[]")
	}
	SanitizedResponse(w, string(ret))
}

func (i *jsonAPIHandler) POSTPublish(w http.ResponseWriter, r *http.Request) {
	// Republish to IPNS
	if err := i.node.SeedNode(); err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	SanitizedResponse(w, "{}")
}

func (i *jsonAPIHandler) POSTPurgeCache(w http.ResponseWriter, r *http.Request) {

	ch, err := i.node.IpfsNode.Blockstore.AllKeysChan(context.Background())
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	for id := range ch {
		if err := i.node.IpfsNode.Blockstore.DeleteBlock(id); err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	// Republish to IPNS
	if err := i.node.SeedNode(); err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	SanitizedResponse(w, "{}")
}

func (i *jsonAPIHandler) GETResolve(w http.ResponseWriter, r *http.Request) {
	_, name := path.Split(r.URL.Path)
	pid, err := i.node.NameSystem.Resolve(context.Background(), name)
	if err != nil {
		ErrorResponse(w, http.StatusNotFound, err.Error())
		return
	}
	fmt.Fprint(w, pid.Pretty())
}

func (i *jsonAPIHandler) GETIPNS(w http.ResponseWriter, r *http.Request) {
	_, peerID := path.Split(r.URL.Path)

	val, err := i.node.IpfsNode.Repo.Datastore().Get(dshelp.NewKeyFromBinary([]byte("/ipns/" + peerID)))
	if err != nil { // No record in datastore
		ErrorResponse(w, http.StatusNotFound, err.Error())
		return
	}
	pid, err := peer.IDB58Decode(peerID)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	var keyBytes []byte
	pubkey := i.node.IpfsNode.Peerstore.PubKey(pid)
	if pubkey == nil || !pid.MatchesPublicKey(pubkey) {
		keyval, err := i.node.IpfsNode.Repo.Datastore().Get(dshelp.NewKeyFromBinary([]byte(core.KeyCachePrefix + peerID)))
		if err != nil {
			ErrorResponse(w, http.StatusNotFound, err.Error())
			return
		}
		keyBytes = keyval.([]byte)
	} else {
		keyBytes, err = pubkey.Bytes()
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	type KeyAndRecord struct {
		Pubkey           string `json:"pubkey"`
		SerializedRecord string `json:"serializedRecord"`
	}

	entry := new(ipnspb.IpnsEntry)
	err = proto.Unmarshal(val.([]byte), entry)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	b, err := proto.Marshal(entry)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	ret := KeyAndRecord{hex.EncodeToString(keyBytes), hex.EncodeToString(b)}
	retBytes, err := json.MarshalIndent(ret, "", "    ")
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	go ipfs.Resolve(i.node.IpfsNode, pid, time.Minute, false)
	fmt.Fprint(w, string(retBytes))
}

func (i *jsonAPIHandler) GETHealthCheck(w http.ResponseWriter, r *http.Request) {
	type resp struct {
		Database bool `json:"database"`
		IPFSRoot bool `json:"ipfsRoot"`
		Peers    bool `json:"peers"`
	}

	re := resp{true, true, true}
	pingErr := i.node.Datastore.Ping()
	if pingErr != nil {
		re.Database = false
	}
	_, ferr := os.Stat(i.node.RepoPath)
	if ferr != nil {
		re.IPFSRoot = false
	}
	peers := ipfs.ConnectedPeers(i.node.IpfsNode)
	if len(peers) == 0 {
		re.Peers = false
	}
	if pingErr != nil || ferr != nil {
		ret, _ := json.MarshalIndent(re, "", "    ")
		ErrorResponse(w, http.StatusNotFound, string(ret))
		return
	}
	SanitizedResponse(w, "{}")
}

func (i *jsonAPIHandler) GETPeerInfo(w http.ResponseWriter, r *http.Request) {
	_, idb58 := path.Split(r.URL.Path)
	pid, err := peer.IDB58Decode(idb58)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	pi, err := i.node.IpfsNode.Routing.FindPeer(ctx, pid)
	if err != nil {
		ErrorResponse(w, http.StatusNotFound, err.Error())
		return
	}
	out, err := pi.MarshalJSON()
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	SanitizedResponse(w, string(out))
}
