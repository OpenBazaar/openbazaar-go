package api

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/OpenBazaar/openbazaar-go/core"
	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/spvwallet"
	btc "github.com/btcsuite/btcutil"
	"github.com/golang/protobuf/jsonpb"
	mh "gx/ipfs/QmYf7ng2hG5XBtJA3tN34DQ2GUN5HNksEw1rLDkmr6vGku/go-multihash"
	ma "gx/ipfs/QmYzDkkgAEmrcNzFCiYo6L1dTX4EAG1gZkbtdbd9trL4vd/go-multiaddr"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path"
	"runtime/debug"
	"strconv"
	"strings"
)

type JsonAPIConfig struct {
	Headers       map[string][]string
	Enabled       bool
	Cors          bool
	Authenticated bool
	CookieJar     []http.Cookie
	Username      string
	Password      string
}

type jsonAPIHandler struct {
	config JsonAPIConfig
	node   *core.OpenBazaarNode
}

func newJsonAPIHandler(node *core.OpenBazaarNode, cookieJar []http.Cookie) (*jsonAPIHandler, error) {
	enabled, err := repo.GetAPIEnabled(path.Join(node.RepoPath, "config"))
	if err != nil {
		log.Error(err)
		return nil, err
	}
	username, password, err := repo.GetAPIUsernameAndPw(path.Join(node.RepoPath, "config"))
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

	cfg, err := node.Context.GetConfig()
	if err != nil {
		return nil, err
	}

	gatewayMaddr, err := ma.NewMultiaddr(cfg.Addresses.Gateway)
	if err != nil {
		return nil, err
	}
	addr, err := gatewayMaddr.ValueForProtocol(ma.P_IP4)
	if err != nil {
		return nil, err
	}
	var authenticated bool
	if addr != "127.0.0.1" {
		authenticated = true
	}

	i := &jsonAPIHandler{
		config: JsonAPIConfig{
			Enabled:       enabled,
			Cors:          cors,
			Headers:       headers,
			Authenticated: authenticated,
			CookieJar:     cookieJar,
			Username:      username,
			Password:      password,
		},
		node: node,
	}
	return i, nil
}

// TODO: Build out the api
func (i *jsonAPIHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !i.config.Enabled {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, "403 - Forbidden")
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

	if i.config.Authenticated {
		cookie, err := r.Cookie("OpenBazaarSession")
		if err != nil {
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprint(w, "403 - Forbidden")
			return
		}
		var auth bool
		for _, key := range i.config.CookieJar {
			if key.Value == cookie.Value {
				auth = true
				break
			}
		}
		endpoint, err := url.Parse(r.URL.Path)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, "403 - Forbidden")
			return
		}
		if !auth && (endpoint.String() != `/ob/login` || endpoint.String() != `/ob/login/`) {
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprint(w, "403 - Forbidden")
			return
		}
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
		deleter(i, u.String(), w, r)
		return
	case "PATCH":
		patch(i, u.String(), w, r)
		return
	}
}

func (i *jsonAPIHandler) POSTProfile(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")

	//p := ProfileParam{}

	// If the profile is already set tell them to use PUT
	profilePath := path.Join(i.node.RepoPath, "root", "profile")
	_, ferr := os.Stat(profilePath)
	if !os.IsNotExist(ferr) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"success": false, "reason": "Profile already exists. Use PUT."}`)
		return
	}

	// Check JSON decoding and add proper indentation
	profile := new(pb.Profile)
	err := jsonpb.Unmarshal(r.Body, profile)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, err)
		return
	}

	// Save to file
	err = i.node.UpdateProfile(profile)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"success": false, "reason": "File Write Error: %s"}`, err)
		return
	}

	// Update followers/following
	err = i.node.UpdateFollow()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"success": false, "reason": "File Write Error: %s"}`, err)
		return
	}

	// Republish to IPNS
	if err := i.node.SeedNode(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"success": false, "reason": "IPNS Error: %s"}`, err)
		return
	}

	// Write profile back out as json
	m := jsonpb.Marshaler{
		EnumsAsInts:  false,
		EmitDefaults: true,
		Indent:       "    ",
		OrigName:     false,
	}
	out, err := m.MarshalToString(profile)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"success": false, "reason": "IPNS Error: %s"}`, err)
		return
	}
	fmt.Fprintf(w, out)
	return
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
func (i *jsonAPIHandler) PUTProfile(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")

	//p := ProfileParam{}

	// If profile isn't set tell them to use POST
	profilePath := path.Join(i.node.RepoPath, "root", "profile")
	_, ferr := os.Stat(profilePath)
	if os.IsNotExist(ferr) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"success": false, "reason": "Profile doesn't exist yet. Use POST."}`)
		return
	}

	// Check JSON decoding and add proper indentation
	profile := new(pb.Profile)
	err := jsonpb.Unmarshal(r.Body, profile)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, err)
		return
	}

	// Save to file
	err = i.node.UpdateProfile(profile)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"success": false, "reason": "File Write Error: %s"}`, err)
		return
	}

	// Update followers/following
	err = i.node.UpdateFollow()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"success": false, "reason": "File Write Error: %s"}`, err)
		return
	}

	// Republish to IPNS
	if err := i.node.SeedNode(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"success": false, "reason": "IPNS Error: %s"}`, err)
		return
	}

	// Return the profile in json format
	m := jsonpb.Marshaler{
		EnumsAsInts:  false,
		EmitDefaults: true,
		Indent:       "    ",
		OrigName:     false,
	}
	out, err := m.MarshalToString(profile)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"success": false, "reason": %s}`, err)
	}
	fmt.Fprintf(w, out)
	return
}

func (i *jsonAPIHandler) PUTAvatar(w http.ResponseWriter, r *http.Request) {
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

	// Add hash to profile
	hash, aerr := ipfs.AddFile(i.node.Context, imgPath)
	if aerr != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, aerr)
		return
	}
	profile, err := i.node.GetProfile()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, aerr)
		return
	}
	profile.AvatarHash = hash
	err = i.node.UpdateProfile(&profile)
	if aerr != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, aerr)
		return
	}

	// Update followers/following
	err = i.node.UpdateFollow()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"success": false, "reason": "File Write Error: %s"}`, err)
		return
	}

	if err := i.node.SeedNode(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, err)
		return
	}
	fmt.Fprintf(w, `{}`)
	return
}

func (i *jsonAPIHandler) PUTHeader(w http.ResponseWriter, r *http.Request) {
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

	// Add hash to profile
	hash, aerr := ipfs.AddFile(i.node.Context, imgPath)
	if aerr != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, aerr)
		return
	}
	profile, err := i.node.GetProfile()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, aerr)
		return
	}
	profile.HeaderHash = hash
	err = i.node.UpdateProfile(&profile)
	if aerr != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, aerr)
		return
	}

	// Update followers/following
	err = i.node.UpdateFollow()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"success": false, "reason": "File Write Error: %s"}`, err)
		return
	}

	if err := i.node.SeedNode(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, err)
		return
	}
	fmt.Fprintf(w, `{}`)
	return
}

func (i *jsonAPIHandler) PUTImage(w http.ResponseWriter, r *http.Request) {
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
	type retImage struct {
		Filename string
		Hash     string
	}
	var retData []retImage
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
		rtimg := retImage{img.Filename, hash}
		retData = append(retData, rtimg)
	}
	jsonHashes, err := json.Marshal(retData)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, err)
		return
	}
	fmt.Fprintf(w, `{"images: "%s"}`, string(jsonHashes))
	return
}

func (i *jsonAPIHandler) POSTListing(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")

	ld := new(pb.ListingReqApi)
	err := jsonpb.Unmarshal(r.Body, ld)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, err)
		return
	}

	// If the listing already exists tell them to use PUT
	listingPath := path.Join(i.node.RepoPath, "root", "listings", ld.Listing.Slug)
	_, ferr := os.Stat(listingPath)
	if !os.IsNotExist(ferr) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"success": false, "reason": "Listing already exists. Use PUT."}`)
		return
	}
	if err := os.MkdirAll(listingPath, os.ModePerm); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, err)
		return
	}
	contract, err := i.node.SignListing(ld.Listing)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, err)
		return
	}
	err = i.node.SetListingInventory(ld.Listing, ld.Inventory)
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
	// Update followers/following
	err = i.node.UpdateFollow()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"success": false, "reason": "File Write Error: %s"}`, err)
		return
	}
	if err := i.node.SeedNode(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, err)
		return
	}
	fmt.Fprintf(w, `{}`)
	return
}

func (i *jsonAPIHandler) PUTListing(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")

	ld := new(pb.ListingReqApi)
	err := jsonpb.Unmarshal(r.Body, ld)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, err)
		return
	}
	listingPath := path.Join(i.node.RepoPath, "root", "listings", ld.Listing.Slug)
	if err := os.MkdirAll(listingPath, os.ModePerm); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, err)
		return
	}
	contract, err := i.node.SignListing(ld.Listing)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, err)
		return
	}
	err = i.node.SetListingInventory(ld.Listing, ld.Inventory)
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
	// Delete existing listing if the slug changed
	if ld.CurrentSlug != "" && ld.CurrentSlug != ld.Listing.Slug {
		err = i.node.TransferImages(ld.CurrentSlug, ld.Listing.Slug)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, err)
			return
		}
		err = i.node.DeleteListing(ld.CurrentSlug)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, err)
			return
		}
	}
	// Update followers/following
	err = i.node.UpdateFollow()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"success": false, "reason": "File Write Error: %s"}`, err)
		return
	}
	if err := i.node.SeedNode(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, err)
		return
	}
	fmt.Fprintf(w, `{}`)
	return
}

func (i *jsonAPIHandler) DELETEListing(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	ld := new(pb.ListingReqApi)
	err := jsonpb.Unmarshal(r.Body, ld)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, err)
		return
	}
	err = i.node.DeleteListing(ld.CurrentSlug)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, err)
		return
	}
	fmt.Fprintf(w, `{}`)
	return
}

func (i *jsonAPIHandler) POSTPurchase(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")

	decoder := json.NewDecoder(r.Body)
	var data core.PurchaseData
	err := decoder.Decode(&data)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, err)
		return
	}
	orderId, paymentAddr, amount, online, err := i.node.Purchase(&data)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, err)
		return
	}
	type purchaseReturn struct {
		PaymentAddress string
		Amount         uint64
		VendorOnline   bool
		OrderId        string
	}
	ret := purchaseReturn{paymentAddr, amount, online, orderId}
	b, err := json.MarshalIndent(ret, "", "    ")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, err)
		return
	}
	fmt.Fprintf(w, string(b))
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
func (i *jsonAPIHandler) GETStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	_, peerId := path.Split(r.URL.Path)
	status := i.node.GetPeerStatus(peerId)
	fmt.Fprintf(w, `{"status": "%s"}`, status)
}

func (i *jsonAPIHandler) GETPeers(w http.ResponseWriter, r *http.Request) {
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

func (i *jsonAPIHandler) POSTFollow(w http.ResponseWriter, r *http.Request) {
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
	fmt.Fprintf(w, `{}`)
	return
}

func (i *jsonAPIHandler) POSTUnfollow(w http.ResponseWriter, r *http.Request) {
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
	fmt.Fprintf(w, `{}`)
	return
}

func (i *jsonAPIHandler) GETAddress(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	addr := i.node.Wallet.CurrentAddress(spvwallet.EXTERNAL)
	fmt.Fprintf(w, `{"address": "%s"}`, addr.EncodeAddress())
}

func (i *jsonAPIHandler) GETMnemonic(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	mn, err := i.node.Datastore.Config().GetMnemonic()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, err)
		return
	}
	fmt.Fprintf(w, `{"mnemonic": "%s"}`, mn)
}

func (i *jsonAPIHandler) GETBalance(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	confirmed, unconfirmed := i.node.Wallet.Balance()
	fmt.Fprintf(w, `{"confirmed": "%d", "unconfirmed": "%d"}`, int(confirmed), int(unconfirmed))
}

func (i *jsonAPIHandler) POSTSpendCoins(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	type Send struct {
		Address  string `json:"address"`
		Amount   int64  `json:"amount"`
		FeeLevel string `json:"feeLevel"`
	}
	decoder := json.NewDecoder(r.Body)
	var snd Send
	err := decoder.Decode(&snd)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, err)
		return
	}
	addr, err := btc.DecodeAddress(snd.Address, i.node.Wallet.Params())
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
	fmt.Fprint(w, `{}`)
	return
}

func (i *jsonAPIHandler) GETConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	fmt.Fprintf(w, `{"guid": "%s", "cryptoCurrency": "%s"}`, i.node.IpfsNode.Identity.Pretty(), i.node.Wallet.CurrencyCode())
}

func (i *jsonAPIHandler) POSTSettings(w http.ResponseWriter, r *http.Request) {
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
	fmt.Fprintf(w, `{}`)
	return
}

func (i *jsonAPIHandler) PUTSettings(w http.ResponseWriter, r *http.Request) {
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
	fmt.Fprintf(w, `{}`)
	return
}

func (i *jsonAPIHandler) GETSettings(w http.ResponseWriter, r *http.Request) {
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

func (i *jsonAPIHandler) PATCHSettings(w http.ResponseWriter, r *http.Request) {
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
	fmt.Fprintf(w, `{}`)
	return
}

func (i *jsonAPIHandler) GETClosestPeers(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	_, peerId := path.Split(r.URL.Path)
	var peerIds []string
	peers, err := ipfs.Query(i.node.Context, peerId)
	if err == nil {
		for _, p := range peers {
			peerIds = append(peerIds, p.Pretty())
		}
	}
	ret, _ := json.MarshalIndent(peerIds, "", "")
	if string(ret) == "null" {
		ret = []byte("[]")
	}
	fmt.Fprintf(w, string(ret))
}

func (i *jsonAPIHandler) GETExchangeRate(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	_, currencyCode := path.Split(r.URL.Path)
	rate, err := i.node.ExchangeRates.GetExchangeRate(strings.ToUpper(currencyCode))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, err)
		return
	}
	fmt.Fprintf(w, `%.2f`, rate)
}

func (i *jsonAPIHandler) GETFollowers(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	offset := r.URL.Query().Get("offsetId")
	limit := r.URL.Query().Get("limit")
	if limit == "" {
		limit = "-1"
	}
	l, err := strconv.ParseInt(limit, 10, 32)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, err)
		return
	}
	followers, err := i.node.Datastore.Followers().Get(offset, int(l))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, err)
		return
	}
	ret, _ := json.MarshalIndent(followers, "", "")
	if string(ret) == "null" {
		ret = []byte("[]")
	}
	fmt.Fprintf(w, string(ret))
}

func (i *jsonAPIHandler) GETFollowing(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	offset := r.URL.Query().Get("offsetId")
	limit := r.URL.Query().Get("limit")
	if limit == "" {
		limit = "-1"
	}
	l, err := strconv.ParseInt(limit, 10, 32)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, err)
		return
	}
	following, err := i.node.Datastore.Following().Get(offset, int(l))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, err)
		return
	}
	ret, _ := json.MarshalIndent(following, "", "")
	if string(ret) == "null" {
		ret = []byte("[]")
	}
	fmt.Fprintf(w, string(ret))
	return
}

func (i *jsonAPIHandler) POSTLogin(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	type Credentials struct {
		Username string
		Password string
	}
	decoder := json.NewDecoder(r.Body)
	var cred Credentials
	err := decoder.Decode(&cred)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, err)
		return
	}
	if cred.Username == i.config.Username && cred.Password == i.config.Password {
		var r []byte = make([]byte, 32)
		_, err := rand.Read(r)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, err)
			return
		}
		cookie := &http.Cookie{Name: "OpenBazaarSession", Value: hex.EncodeToString(r)}
		i.config.CookieJar = append(i.config.CookieJar, *cookie)
		http.SetCookie(w, cookie)
	}
	fmt.Fprintf(w, `{}`)
	return
}

func (i *jsonAPIHandler) GETInventory(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	type inv struct {
		Slug  string `json:"slug"`
		Count int    `json:"count"`
	}
	var invList []inv
	inventory, err := i.node.Datastore.Inventory().GetAll()
	if err != nil {
		fmt.Fprintf(w, `[]`)
	}
	for k, v := range inventory {
		i := inv{k, v}
		invList = append(invList, i)
	}
	ret, _ := json.MarshalIndent(invList, "", "")
	if string(ret) == "null" {
		ret = []byte("[]")
	}
	fmt.Fprintf(w, string(ret))
	return
}

func (i *jsonAPIHandler) POSTInventory(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	type inv struct {
		Slug  string
		Count int
	}
	decoder := json.NewDecoder(r.Body)
	var invList []inv
	err := decoder.Decode(&invList)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, err)
		return
	}
	for _, in := range invList {
		err := i.node.Datastore.Inventory().Put(in.Slug, in.Count)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, err)
			return
		}
	}
	fmt.Fprintf(w, `{}`)
	return
}

func (i *jsonAPIHandler) POSTModerator(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	// If the moderator is already set tell them to use PUT
	modPath := path.Join(i.node.RepoPath, "root", "moderation")
	_, ferr := os.Stat(modPath)
	if !os.IsNotExist(ferr) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"success": false, "reason": "Moderator file already exists. Use PUT."}`)
		return
	}

	// Check JSON decoding and add proper indentation
	moderator := new(pb.Moderator)
	err := jsonpb.Unmarshal(r.Body, moderator)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, err)
		return
	}

	// Save self as moderator
	err = i.node.SetSelfAsModerator(moderator)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, err)
		return
	}

	// Update followers/following
	err = i.node.UpdateFollow()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"success": false, "reason": "File Write Error: %s"}`, err)
		return
	}

	// Republish to IPNS
	if err := i.node.SeedNode(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"success": false, "reason": "IPNS Error: %s"}`, err)
		return
	}
	fmt.Fprintf(w, "{}")
	return
}

func (i *jsonAPIHandler) PUTModerator(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	// If the moderator is already set tell them to use PUT
	modPath := path.Join(i.node.RepoPath, "root", "moderation")
	_, ferr := os.Stat(modPath)
	if os.IsNotExist(ferr) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"success": false, "reason": "Moderator file doesn't yet exist. Use POST."}`)
		return
	}

	// Check JSON decoding and add proper indentation
	moderator := new(pb.Moderator)
	err := jsonpb.Unmarshal(r.Body, moderator)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, err)
		return
	}

	// Save self as moderator
	err = i.node.SetSelfAsModerator(moderator)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, err)
		return
	}

	// Update followers/following
	err = i.node.UpdateFollow()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"success": false, "reason": "File Write Error: %s"}`, err)
		return
	}

	// Republish to IPNS
	if err := i.node.SeedNode(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"success": false, "reason": "IPNS Error: %s"}`, err)
		return
	}
	fmt.Fprintf(w, "{}")
	return
}

func (i *jsonAPIHandler) DELETEModerator(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	// If the moderator is already set tell them to use PUT
	modPath := path.Join(i.node.RepoPath, "root", "moderation")
	_, ferr := os.Stat(modPath)
	if os.IsNotExist(ferr) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"success": false, "reason": "This node isn't set as a moderator"}`)
		return
	}

	// Save self as moderator
	err := i.node.RemoveSelfAsModerator()
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, err)
		return
	}

	// Update followers/following
	err = i.node.UpdateFollow()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"success": false, "reason": "File Write Error: %s"}`, err)
		return
	}

	// Republish to IPNS
	if err := i.node.SeedNode(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"success": false, "reason": "IPNS Error: %s"}`, err)
		return
	}
	fmt.Fprintf(w, "{}")
	return
}

func (i *jsonAPIHandler) GETListing(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	contract := new(pb.RicardianContract)
	inventory := []*pb.Inventory{}
	_, listingID := path.Split(r.URL.Path)
	_, err := mh.FromB58String(listingID)
	if err == nil {
		contract, inventory, err = i.node.GetListingFromHash(listingID)
	} else {
		contract, inventory, err = i.node.GetListingFromSlug(listingID)
	}
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, err)
		return
	}
	m := jsonpb.Marshaler{
		EnumsAsInts:  false,
		EmitDefaults: false,
		Indent:       "    ",
		OrigName:     false,
	}
	resp := new(pb.ListingRespApi)
	resp.Contract = contract
	resp.Inventory = inventory
	out, err := m.MarshalToString(resp)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"success": false, "reason": "%s"}`, err)
		return
	}
	fmt.Fprintf(w, string(out))
	return
}
