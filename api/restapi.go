package api

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path"
	"runtime/debug"
	"strings"

	"github.com/OpenBazaar/openbazaar-go/core"
	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/spvwallet"
	btc "github.com/btcsuite/btcutil"
	"github.com/golang/protobuf/jsonpb"
	"github.com/ipfs/go-ipfs/core/corehttp"
	"github.com/OpenBazaar/openbazaar-go/repo"
)

type RestAPIConfig struct {
	Headers      map[string][]string
	BlockList    *corehttp.BlockList
	Enabled      bool
	Cors         bool
}

type restAPIHandler struct {
	config RestAPIConfig
	node   *core.OpenBazaarNode
}

func newRestAPIHandler(node *core.OpenBazaarNode) (*restAPIHandler, error) {

	// Add the current node directory in case it's note already added.
	dirHash, aerr := ipfs.AddDirectory(node.Context, path.Join(node.RepoPath, "root"))
	if aerr != nil {
		log.Error(aerr)
		return nil, aerr
	}
	node.RootHash = dirHash

	enabled, err := repo.GetAPIEnabled(path.Join(node.RepoPath, "config"))
	if err != nil {
		log.Error(err)
		return nil, err
	}
	cors, err := repo.GetAPICORS(path.Join(node.RepoPath, "config"))
	if err != nil {
		log.Error(err)
		return nil, err
	}
	headers, err := repo.GetAPIHeaders(path.Join(node.RepoPath, "config"))
	if err != nil {
		log.Error(err)
		return nil, err
	}
	i := &restAPIHandler{
		config: RestAPIConfig{
			Enabled: enabled,
			Cors: cors,
			Headers: headers,
			BlockList:    &corehttp.BlockList{},
		},
		node: node,
	}
	return i, nil
}

// TODO: Build out the api
func (i *restAPIHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !i.config.Enabled{
		w.WriteHeader(http.StatusMethodNotAllowed)
		fmt.Fprint(w, "api access disallowed")
		return
	}
	if i.config.Cors {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "PUT,POST,DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
	}

	for k, v := range i.config.Headers {
		w.Header()[k] = v
	}

	// Stop here if its Preflighted OPTIONS request
	if r.Method == "OPTIONS" {
		return
	}
	dump, err := httputil.DumpRequest(r, false)
	if err != nil {
		log.Error("Error reading http request:", err)
	}
	log.Debugf("%s", dump)
	defer func() {
		if r := recover(); r != nil {
			log.Error("A panic occurred in the rest api handler!")
			log.Error(r)
			debug.PrintStack()
		}
	}()

	u, err := url.Parse(r.URL.Path)
	if err != nil {
		panic(err)
	}
	switch r.Method {
	case "GET":
		get(i, u.String(), w, r)
		return
	case "POST":
		post(i, u.String(), w, r)
		return
	case "PUT":
		put(i, u.String(), w, r)
		return
	case "DELETE":
		// TODO: not yet implemented
		return
	case "PATCH":
		patch(i, u.String(), w, r)
		return
	}
}

func (i *restAPIHandler) POSTProfile(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")

	//p := ProfileParam{}

	profilePath := path.Join(i.node.RepoPath, "root", "profile")
	if _, err := os.Stat(profilePath); !os.IsNotExist(err) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, `{"success": false, "reason": Profile already exists}`)
	}

	// Create profile file
	f, err := os.Create(profilePath)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"success": false, "reason": %s}`, err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			panic(err)
		}
	}()

	// Check JSON decoding and add proper indentation
	dec := json.NewDecoder(r.Body)
	for {
		var v map[string]interface{}
		err := dec.Decode(&v)
		if err == io.EOF {
			break
		}
		b, err := json.MarshalIndent(v, "", "    ")
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, `{"success": false, "reason": "JSON marshalling error: %s"}`, err)
			return
		}
		if _, err := f.WriteString(string(b)); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, `{"success": false, "reason": "File Write Error: %s"}`, err)
			return
		}
	}

	// Republish to IPNS
	if err := i.node.SeedNode(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"success": false, "reason": "IPNS Error: %s"}`, err)
		return
	}
	fmt.Fprintf(w, `{"guid": "%s"}`, i.node.IpfsNode.Identity.Pretty())
}

// swagger:route PUT /profile putProfile
//
// Update profile
//
// This will update the profile file and then re-publish
// to IPNS for consumption by other peers.
//
//     Consumes:
//     - application/json
//
//     Produces:
//     - application/json
//
//     Schemes: http, https
//
//     Security:
//
//     Responses:
//       default: ProfileResponse
//	 200: ProfileResponse
func (i *restAPIHandler) PUTProfile(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")

	//p := ProfileParam{}

	profilePath := path.Join(i.node.RepoPath, "root", "profile")
	if _, err := os.Stat(profilePath); os.IsNotExist(err) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, `{"success": false, "reason": Profile doesn't exist}`)
	}

	// Create profile file
	f, err := os.Create(profilePath)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"success": false, "reason": %s}`, err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			panic(err)
		}
	}()

	// Check JSON decoding and add proper indentation
	dec := json.NewDecoder(r.Body)
	for {
		var v map[string]interface{}
		err := dec.Decode(&v)
		if err == io.EOF {
			break
		}
		b, err := json.MarshalIndent(v, "", "    ")
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, `{"success": false, "reason": "JSON marshalling error: %s"}`, err)
			return
		}
		if _, err := f.WriteString(string(b)); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, `{"success": false, "reason": "File Write Error: %s"}`, err)
			return
		}
	}

	// Republish to IPNS
	if err := i.node.SeedNode(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"success": false, "reason": "IPNS Error: %s"}`, err)
		return
	}
	return
}

func (i *restAPIHandler) PUTAvatar(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	type ImgData struct {
		Avatar string
	}
	decoder := json.NewDecoder(r.Body)
	data := new(ImgData)
	err := decoder.Decode(&data)

	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, err)
		return
	}
	imgPath := path.Join(i.node.RepoPath, "root", "avatar")
	out, err := os.Create(imgPath)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, err)
		return
	}

	dec := base64.NewDecoder(base64.StdEncoding, strings.NewReader(data.Avatar))

	defer out.Close()

	_, err = io.Copy(out, dec)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, err)
		return
	}

	if err := i.node.SeedNode(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, err)
		return
	}

	return
}

func (i *restAPIHandler) PUTHeader(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	type ImgData struct {
		Header string
	}
	decoder := json.NewDecoder(r.Body)
	data := new(ImgData)
	err := decoder.Decode(&data)

	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, err)
		return
	}
	imgPath := path.Join(i.node.RepoPath, "root", "header")
	out, err := os.Create(imgPath)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, err)
		return
	}

	dec := base64.NewDecoder(base64.StdEncoding, strings.NewReader(data.Header))

	defer out.Close()

	_, err = io.Copy(out, dec)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, err)
		return
	}

	if err := i.node.SeedNode(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, err)
		return
	}
	return
}

func (i *restAPIHandler) PUTImage(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	type ImgData struct {
		Directory string
		Filename  string
		Image     string
	}
	decoder := json.NewDecoder(r.Body)
	var images []ImgData
	err := decoder.Decode(&images)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, err)
		return
	}
	var imageHashes []string
	for _, img := range images {
		if err := os.MkdirAll(path.Join(i.node.RepoPath, "root", "listings", img.Directory), os.ModePerm); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, err)
			return
		}
		imgPath := path.Join(i.node.RepoPath, "root", "listings", img.Directory, img.Filename)
		out, err := os.Create(imgPath)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, err)
			return
		}

		dec := base64.NewDecoder(base64.StdEncoding, strings.NewReader(img.Image))

		defer out.Close()

		_, err = io.Copy(out, dec)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, err)
			return
		}
		hash, aerr := ipfs.AddFile(i.node.Context, imgPath)
		if aerr != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, aerr)
			return
		}
		imageHashes = append(imageHashes, hash+" "+img.Filename)
	}
	jsonHashes, err := json.Marshal(imageHashes)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, err)
		return
	}
	fmt.Fprintf(w, `{"hashes: "%s"}`, string(jsonHashes))
}

func (i *restAPIHandler) POSTListing(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")

	l := new(pb.Listing)
	jsonpb.Unmarshal(r.Body, l)
	listingPath := path.Join(i.node.RepoPath, "root", "listings", l.ListingName)
	if err := os.MkdirAll(listingPath, os.ModePerm); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, err)
		return
	}

	contract, err := i.node.SignListing(l)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, err)
		return
	}
	f, err := os.Create(path.Join(listingPath, "listing.json"))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, err)
		return
	}
	defer func() {
		if err := f.Close(); err != nil {
			panic(err)
		}
	}()

	m := jsonpb.Marshaler{
		EnumsAsInts:  false,
		EmitDefaults: false,
		Indent:       "    ",
		OrigName:     false,
	}
	out, err := m.MarshalToString(contract)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, err)
		return
	}

	if _, err := f.WriteString(out); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, err)
		return
	}
	err = i.node.UpdateListingIndex(contract)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, err)
		return
	}

	if err := i.node.SeedNode(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, err)
		return
	}
	return
}

func (i *restAPIHandler) POSTPurchase(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")

	decoder := json.NewDecoder(r.Body)
	var data core.PurchaseData
	err := decoder.Decode(&data)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, err)
		return
	}
	if err := i.node.Purchase(&data); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, err)
		return
	}
	return
}

// swagger:route GET /status/{PeerId} status
//
// Get Status of Peer
//
// This will give you the status of a specific peer by id
//
//     Consumes:
//     - application/json
//
//     Produces:
//     - application/json
//
//     Schemes: http, https
//
//
//     Responses:
//       default: StatusResponse
//	 200: StatusResponse
//
func (i *restAPIHandler) GETStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	s := PeerIdParam{}
	_, peerId := path.Split(r.URL.Path)
	s.PeerId = peerId
	status := i.node.GetPeerStatus(s.PeerId)
	fmt.Fprintf(w, `{"status": "%s"}`, status)
}

func (i *restAPIHandler) GETPeers(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	peers, err := ipfs.ConnectedPeers(i.node.Context)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, err)
		return
	}

	peerJson, err := json.MarshalIndent(peers, "", "    ")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, err)
		return
	}
	fmt.Fprintf(w, string(peerJson))
}

func (i *restAPIHandler) POSTFollow(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	type PeerId struct {
		ID string
	}
	decoder := json.NewDecoder(r.Body)
	var pid PeerId
	err := decoder.Decode(&pid)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, err)
		return
	}
	if err := i.node.Follow(pid.ID); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, err)
		return
	}
	return
}

func (i *restAPIHandler) POSTUnfollow(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	type PeerId struct {
		ID string
	}
	decoder := json.NewDecoder(r.Body)
	var pid PeerId
	err := decoder.Decode(&pid)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, err)
		return
	}
	if err := i.node.Unfollow(pid.ID); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, err)
		return
	}
	return
}

func (i *restAPIHandler) GETAddress(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	addr := i.node.Wallet.CurrentAddress(spvwallet.EXTERNAL)
	fmt.Fprintf(w, `{"address": "%s"}`, addr.EncodeAddress())
}

func (i *restAPIHandler) GETMnemonic(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	mn, err := i.node.Datastore.Config().GetMnemonic()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, err)
		return
	}
	fmt.Fprintf(w, `{"mnemonic": "%s"}`, mn)
}

func (i *restAPIHandler) GETBalance(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	confirmed, unconfirmed := i.node.Wallet.Balance()
	fmt.Fprintf(w, `{"confirmed": "%d", "unconfirmed": "%d"}`, int(confirmed), int(unconfirmed))
}

func (i *restAPIHandler) POSTSpendCoins(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	type Send struct {
		Address  string
		Amount   int64
		FeeLevel string
	}
	decoder := json.NewDecoder(r.Body)
	var snd Send
	err := decoder.Decode(&snd)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, err)
		return
	}
	addr, err := btc.DecodeAddress("2ND3E7kix3xogR77H8F35zjufXZkHUvWTvA", i.node.Wallet.Params())
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, err)
		return
	}
	var feeLevel spvwallet.FeeLevel
	switch strings.ToUpper(snd.FeeLevel) {
	case "PRIORITY":
		feeLevel = spvwallet.PRIOIRTY
	case "NORMAL":
		feeLevel = spvwallet.NORMAL
	case "Economic":
		feeLevel = spvwallet.ECONOMIC
	}
	if err := i.node.Wallet.Spend(snd.Amount, addr, feeLevel); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, err)
		return
	}
	return
}

func (i *restAPIHandler) GETConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	fmt.Fprintf(w, `{"guid": "%s"}`, i.node.IpfsNode.Identity.Pretty())
}

func (i *restAPIHandler) POSTSettings(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	var settings repo.SettingsData
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&settings)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, err)
		return
	}
	_, err = i.node.Datastore.Settings().Get()
	if err == nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, `{"success": false, "reason": "Settings is already set. Use PUT."}`)
		return
	}
	err = i.node.Datastore.Settings().Put(settings)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, err)
		return
	}
	return
}

func (i *restAPIHandler) PUTSettings(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	var settings repo.SettingsData
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&settings)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, err)
		return
	}
	_, err = i.node.Datastore.Settings().Get()
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, `{"success": false, "reason": "Settings is not yet set. Use POST."}`)
		return
	}
	err = i.node.Datastore.Settings().Put(settings)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, err)
		return
	}
	return
}

func (i *restAPIHandler) GETSettings(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	settings, err := i.node.Datastore.Settings().Get()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, err)
		return
	}
	settingsJson, err := json.MarshalIndent(&settings, "", "    ")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, err)
		return
	}
	fmt.Fprintf(w, string(settingsJson))
}

func (i *restAPIHandler) PATCHSettings(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	var settings repo.SettingsData
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&settings)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, err)
		return
	}
	err = i.node.Datastore.Settings().Update(settings)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, err)
		return
	}
	return
}