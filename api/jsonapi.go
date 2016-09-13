package api

import (
	"encoding/base64"
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
	Cookie        http.Cookie
}

type jsonAPIHandler struct {
	config JsonAPIConfig
	node   *core.OpenBazaarNode
}

func newJsonAPIHandler(node *core.OpenBazaarNode, authenticated bool, authCookie http.Cookie) (*jsonAPIHandler, error) {
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

	i := &jsonAPIHandler{
		config: JsonAPIConfig{
			Enabled:       enabled,
			Cors:          cors,
			Headers:       headers,
			Authenticated: authenticated,
			Cookie:        authCookie,
		},
		node: node,
	}
	return i, nil
}

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
		cookie, err := r.Cookie("OpenBazaar_Auth_Cookie")
		if err != nil {
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprint(w, "403 - Forbidden")
			return
		}
		if i.config.Cookie.Value != cookie.Value {
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
	w.Header().Add("Content-Type", "application/json")
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

func ErrorResponse(w http.ResponseWriter, errorCode int, reason string) {
	type ApiError struct {
		Success bool   `json:"success"`
		Reason  string `json:"reason"`
	}
	reason = strings.Replace(reason, `"`, `'`, -1)
	err := ApiError{false, reason}
	resp, _ := json.MarshalIndent(err, "", "    ")
	w.WriteHeader(errorCode)
	fmt.Fprint(w, string(resp))
}

func (i *jsonAPIHandler) POSTProfile(w http.ResponseWriter, r *http.Request) {

	// If the profile is already set tell them to use PUT
	profilePath := path.Join(i.node.RepoPath, "root", "profile")
	_, ferr := os.Stat(profilePath)
	if !os.IsNotExist(ferr) {
		ErrorResponse(w, http.StatusConflict, "Profile already exists. Use PUT.")
		return
	}

	// Check JSON decoding and add proper indentation
	profile := new(pb.Profile)
	err := jsonpb.Unmarshal(r.Body, profile)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	// Save to file
	err = i.node.UpdateProfile(profile)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Update followers/following
	err = i.node.UpdateFollow()
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, "File Write Error: "+err.Error())
		return
	}

	// Republish to IPNS
	if err := i.node.SeedNode(); err != nil {
		ErrorResponse(w, http.StatusInternalServerError, "IPNS Error: "+err.Error())
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
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	fmt.Fprint(w, out)
	return
}

func (i *jsonAPIHandler) PUTProfile(w http.ResponseWriter, r *http.Request) {

	// If profile isn't set tell them to use POST
	profilePath := path.Join(i.node.RepoPath, "root", "profile")
	_, ferr := os.Stat(profilePath)
	if os.IsNotExist(ferr) {
		ErrorResponse(w, http.StatusNotFound, "Profile doesn't exist yet. Use POST.")
		return
	}

	// Check JSON decoding and add proper indentation
	profile := new(pb.Profile)
	err := jsonpb.Unmarshal(r.Body, profile)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	// Save to file
	err = i.node.UpdateProfile(profile)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Update followers/following
	err = i.node.UpdateFollow()
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Republish to IPNS
	if err := i.node.SeedNode(); err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
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
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	fmt.Fprint(w, out)
	return
}

func (i *jsonAPIHandler) POSTAvatar(w http.ResponseWriter, r *http.Request) {
	type ImgData struct {
		Avatar string
	}
	decoder := json.NewDecoder(r.Body)
	data := new(ImgData)
	err := decoder.Decode(&data)

	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	imgPath := path.Join(i.node.RepoPath, "root", "avatar")
	out, err := os.Create(imgPath)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	dec := base64.NewDecoder(base64.StdEncoding, strings.NewReader(data.Avatar))

	defer out.Close()

	_, err = io.Copy(out, dec)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Add hash to profile
	hash, aerr := ipfs.AddFile(i.node.Context, imgPath)
	if aerr != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	profile, err := i.node.GetProfile()
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	profile.AvatarHash = hash
	err = i.node.UpdateProfile(&profile)
	if aerr != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Update followers/following
	err = i.node.UpdateFollow()
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err := i.node.SeedNode(); err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	fmt.Fprint(w, `{}`)
	return
}

func (i *jsonAPIHandler) POSTHeader(w http.ResponseWriter, r *http.Request) {
	type ImgData struct {
		Header string
	}
	decoder := json.NewDecoder(r.Body)
	data := new(ImgData)
	err := decoder.Decode(&data)

	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	imgPath := path.Join(i.node.RepoPath, "root", "header")
	out, err := os.Create(imgPath)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	dec := base64.NewDecoder(base64.StdEncoding, strings.NewReader(data.Header))

	defer out.Close()

	_, err = io.Copy(out, dec)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Add hash to profile
	hash, aerr := ipfs.AddFile(i.node.Context, imgPath)
	if aerr != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	profile, err := i.node.GetProfile()
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	profile.HeaderHash = hash
	err = i.node.UpdateProfile(&profile)
	if aerr != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Update followers/following
	err = i.node.UpdateFollow()
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, "File write error: "+err.Error())
		return
	}

	if err := i.node.SeedNode(); err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	fmt.Fprint(w, `{}`)
	return
}

func (i *jsonAPIHandler) POSTImage(w http.ResponseWriter, r *http.Request) {
	type ImgData struct {
		Filename string `json:"filename"`
		Image    string `json:"image"`
	}
	decoder := json.NewDecoder(r.Body)
	var images []ImgData
	err := decoder.Decode(&images)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	type retImage struct {
		Filename string `json:"filename"`
		Hash     string `json:"hash"`
	}
	var retData []retImage
	for _, img := range images {
		imgPath := path.Join(i.node.RepoPath, "root", "images", img.Filename)
		out, err := os.Create(imgPath)
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}

		dec := base64.NewDecoder(base64.StdEncoding, strings.NewReader(img.Image))

		defer out.Close()

		_, err = io.Copy(out, dec)
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		hash, aerr := ipfs.AddFile(i.node.Context, imgPath)
		if aerr != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		rtimg := retImage{img.Filename, hash}
		retData = append(retData, rtimg)
	}
	jsonHashes, err := json.MarshalIndent(retData, "", "    ")
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	fmt.Fprint(w, string(jsonHashes))
	return
}

func (i *jsonAPIHandler) POSTListing(w http.ResponseWriter, r *http.Request) {
	ld := new(pb.ListingReqApi)
	err := jsonpb.Unmarshal(r.Body, ld)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	// If the listing already exists tell them to use PUT
	listingPath := path.Join(i.node.RepoPath, "root", "listings", ld.Listing.Slug+".json")
	_, ferr := os.Stat(listingPath)
	if !os.IsNotExist(ferr) {
		ErrorResponse(w, http.StatusConflict, "Listing already exists. Use PUT.")
		return
	}
	contract, err := i.node.SignListing(ld.Listing)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	err = i.node.SetListingInventory(ld.Listing, ld.Inventory)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	f, err := os.Create(listingPath)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
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
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	if _, err := f.WriteString(out); err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	err = i.node.UpdateListingIndex(contract)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	// Update followers/following
	err = i.node.UpdateFollow()
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := i.node.SeedNode(); err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	fmt.Fprint(w, `{}`)
	return
}

func (i *jsonAPIHandler) PUTListing(w http.ResponseWriter, r *http.Request) {
	ld := new(pb.ListingReqApi)
	err := jsonpb.Unmarshal(r.Body, ld)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	listingPath := path.Join(i.node.RepoPath, "root", "listings", ld.Listing.Slug+".json")
	contract, err := i.node.SignListing(ld.Listing)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	err = i.node.SetListingInventory(ld.Listing, ld.Inventory)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	f, err := os.Create(listingPath)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
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
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	if _, err := f.WriteString(out); err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	err = i.node.UpdateListingIndex(contract)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	// Delete existing listing if the slug changed
	if ld.CurrentSlug != "" && ld.CurrentSlug != ld.Listing.Slug {
		err := i.node.DeleteListing(ld.CurrentSlug)
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, "File Write Error: "+err.Error())
			return
		}
	}
	// Update followers/following
	err = i.node.UpdateFollow()
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, "File Write Error: "+err.Error())
		return
	}
	if err := i.node.SeedNode(); err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	fmt.Fprint(w, `{}`)
	return
}

func (i *jsonAPIHandler) DELETEListing(w http.ResponseWriter, r *http.Request) {
	ld := new(pb.ListingReqApi)
	err := jsonpb.Unmarshal(r.Body, ld)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	err = i.node.DeleteListing(ld.CurrentSlug)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	fmt.Fprint(w, `{}`)
	return
}

func (i *jsonAPIHandler) POSTPurchase(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	var data core.PurchaseData
	err := decoder.Decode(&data)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	orderId, paymentAddr, amount, online, err := i.node.Purchase(&data)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
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
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	fmt.Fprint(w, string(b))
	return
}

func (i *jsonAPIHandler) GETStatus(w http.ResponseWriter, r *http.Request) {
	_, peerId := path.Split(r.URL.Path)
	status := i.node.GetPeerStatus(peerId)
	fmt.Fprintf(w, `{"status": "%s"}`, status)
}

func (i *jsonAPIHandler) GETPeers(w http.ResponseWriter, r *http.Request) {
	peers, err := ipfs.ConnectedPeers(i.node.Context)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	peerJson, err := json.MarshalIndent(peers, "", "    ")
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	fmt.Fprint(w, string(peerJson))
}

func (i *jsonAPIHandler) POSTFollow(w http.ResponseWriter, r *http.Request) {
	type PeerId struct {
		ID string `json:"id"`
	}

	decoder := json.NewDecoder(r.Body)
	var pid PeerId
	err := decoder.Decode(&pid)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := i.node.Follow(pid.ID); err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	fmt.Fprint(w, `{}`)
	return
}

func (i *jsonAPIHandler) POSTUnfollow(w http.ResponseWriter, r *http.Request) {
	type PeerId struct {
		ID string `json:"id"`
	}
	decoder := json.NewDecoder(r.Body)
	var pid PeerId
	err := decoder.Decode(&pid)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := i.node.Unfollow(pid.ID); err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	fmt.Fprint(w, `{}`)
	return
}

func (i *jsonAPIHandler) GETAddress(w http.ResponseWriter, r *http.Request) {
	addr := i.node.Wallet.CurrentAddress(spvwallet.EXTERNAL)
	fmt.Fprintf(w, `{"address": "%s"}`, addr.EncodeAddress())
}

func (i *jsonAPIHandler) GETMnemonic(w http.ResponseWriter, r *http.Request) {
	mn, err := i.node.Datastore.Config().GetMnemonic()
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	fmt.Fprintf(w, `{"mnemonic": "%s"}`, mn)
}

func (i *jsonAPIHandler) GETBalance(w http.ResponseWriter, r *http.Request) {
	confirmed, unconfirmed := i.node.Wallet.Balance()
	fmt.Fprintf(w, `{"confirmed": "%d", "unconfirmed": "%d"}`, int(confirmed), int(unconfirmed))
}

func (i *jsonAPIHandler) POSTSpendCoins(w http.ResponseWriter, r *http.Request) {
	type Send struct {
		Address  string `json:"address"`
		Amount   int64  `json:"amount"`
		FeeLevel string `json:"feeLevel"`
	}
	decoder := json.NewDecoder(r.Body)
	var snd Send
	err := decoder.Decode(&snd)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	addr, err := btc.DecodeAddress(snd.Address, i.node.Wallet.Params())
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	var feeLevel spvwallet.FeeLevel
	switch strings.ToUpper(snd.FeeLevel) {
	case "PRIORITY":
		feeLevel = spvwallet.PRIOIRTY
	case "NORMAL":
		feeLevel = spvwallet.NORMAL
	case "ECONOMIC":
		feeLevel = spvwallet.ECONOMIC
	}
	if err := i.node.Wallet.Spend(snd.Amount, addr, feeLevel); err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	fmt.Fprint(w, `{}`)
	return
}

func (i *jsonAPIHandler) GETConfig(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, `{"guid": "%s", "cryptoCurrency": "%s"}`, i.node.IpfsNode.Identity.Pretty(), i.node.Wallet.CurrencyCode())
}

func (i *jsonAPIHandler) POSTSettings(w http.ResponseWriter, r *http.Request) {
	var settings repo.SettingsData
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&settings)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	_, err = i.node.Datastore.Settings().Get()
	if err == nil {
		ErrorResponse(w, http.StatusConflict, "Settings is already set. Use PUT.")
		return
	}
	err = i.node.Datastore.Settings().Put(settings)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	fmt.Fprint(w, `{}`)
	return
}

func (i *jsonAPIHandler) PUTSettings(w http.ResponseWriter, r *http.Request) {
	var settings repo.SettingsData
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&settings)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	_, err = i.node.Datastore.Settings().Get()
	if err != nil {
		ErrorResponse(w, http.StatusNotFound, "Settings is not yet set. Use POST.")
		return
	}
	err = i.node.Datastore.Settings().Put(settings)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	fmt.Fprint(w, `{}`)
	return
}

func (i *jsonAPIHandler) GETSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := i.node.Datastore.Settings().Get()
	if err != nil {
		ErrorResponse(w, http.StatusNotFound, err.Error())
		return
	}
	settingsJson, err := json.MarshalIndent(&settings, "", "    ")
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	fmt.Fprint(w, string(settingsJson))
}

func (i *jsonAPIHandler) PATCHSettings(w http.ResponseWriter, r *http.Request) {
	var settings repo.SettingsData
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&settings)
	if err != nil {
		switch err.Error() {
		case "Not Found":
			ErrorResponse(w, http.StatusNotFound, err.Error())
		default:
			ErrorResponse(w, http.StatusBadRequest, err.Error())
		}
		return
	}
	err = i.node.Datastore.Settings().Update(settings)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	fmt.Fprint(w, `{}`)
}

func (i *jsonAPIHandler) GETClosestPeers(w http.ResponseWriter, r *http.Request) {
	_, peerId := path.Split(r.URL.Path)
	var peerIds []string
	peers, err := ipfs.Query(i.node.Context, peerId)
	if err == nil {
		for _, p := range peers {
			peerIds = append(peerIds, p.Pretty())
		}
	}
	ret, _ := json.MarshalIndent(peerIds, "", "    ")
	if string(ret) == "null" {
		ret = []byte("[]")
	}
	fmt.Fprint(w, string(ret))
}

func (i *jsonAPIHandler) GETExchangeRate(w http.ResponseWriter, r *http.Request) {
	_, currencyCode := path.Split(r.URL.Path)
	rate, err := i.node.ExchangeRates.GetExchangeRate(strings.ToUpper(currencyCode))
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	fmt.Fprintf(w, `%.2f`, rate)
}

func (i *jsonAPIHandler) GETFollowers(w http.ResponseWriter, r *http.Request) {
	offset := r.URL.Query().Get("offsetId")
	limit := r.URL.Query().Get("limit")
	if limit == "" {
		limit = "-1"
	}
	l, err := strconv.ParseInt(limit, 10, 32)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	followers, err := i.node.Datastore.Followers().Get(offset, int(l))
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	ret, _ := json.MarshalIndent(followers, "", "    ")
	if string(ret) == "null" {
		ret = []byte("[]")
	}
	fmt.Fprint(w, string(ret))
}

func (i *jsonAPIHandler) GETFollowing(w http.ResponseWriter, r *http.Request) {
	offset := r.URL.Query().Get("offsetId")
	limit := r.URL.Query().Get("limit")
	if limit == "" {
		limit = "-1"
	}
	l, err := strconv.ParseInt(limit, 10, 32)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	following, err := i.node.Datastore.Following().Get(offset, int(l))
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	ret, _ := json.MarshalIndent(following, "", "    ")
	if string(ret) == "null" {
		ret = []byte("[]")
	}
	fmt.Fprint(w, string(ret))
	return
}

func (i *jsonAPIHandler) GETInventory(w http.ResponseWriter, r *http.Request) {
	type inv struct {
		Slug     string `json:"slug"`
		Quantity int    `json:"quantity"`
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
	ret, _ := json.MarshalIndent(invList, "", "    ")
	if string(ret) == "null" {
		ret = []byte("[]")
	}
	fmt.Fprint(w, string(ret))
	return
}

func (i *jsonAPIHandler) POSTInventory(w http.ResponseWriter, r *http.Request) {
	type inv struct {
		Slug     string `json:"slug"`
		Quantity int    `json:"quantity"`
	}
	decoder := json.NewDecoder(r.Body)
	var invList []inv
	err := decoder.Decode(&invList)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	for _, in := range invList {
		err := i.node.Datastore.Inventory().Put(in.Slug, in.Quantity)
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	fmt.Fprint(w, `{}`)
	return
}

func (i *jsonAPIHandler) POSTModerator(w http.ResponseWriter, r *http.Request) {
	// If the moderator is already set tell them to use PUT
	modPath := path.Join(i.node.RepoPath, "root", "moderation")
	_, ferr := os.Stat(modPath)
	if !os.IsNotExist(ferr) {
		ErrorResponse(w, http.StatusConflict, "Moderator file already exists. Use PUT.")
		return
	}

	// Check JSON decoding and add proper indentation
	moderator := new(pb.Moderator)
	err := jsonpb.Unmarshal(r.Body, moderator)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	// Save self as moderator
	err = i.node.SetSelfAsModerator(moderator)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Update followers/following
	err = i.node.UpdateFollow()
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Republish to IPNS
	if err := i.node.SeedNode(); err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	fmt.Fprint(w, "{}")
	return
}

func (i *jsonAPIHandler) PUTModerator(w http.ResponseWriter, r *http.Request) {
	// If the moderator is already set tell them to use PUT
	modPath := path.Join(i.node.RepoPath, "root", "moderation")
	_, ferr := os.Stat(modPath)
	if os.IsNotExist(ferr) {
		ErrorResponse(w, http.StatusNotFound, "Moderator file doesn't yet exist. Use POST.")
		return
	}

	// Check JSON decoding and add proper indentation
	moderator := new(pb.Moderator)
	err := jsonpb.Unmarshal(r.Body, moderator)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	// Save self as moderator
	err = i.node.SetSelfAsModerator(moderator)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Update followers/following
	err = i.node.UpdateFollow()
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Republish to IPNS
	if err := i.node.SeedNode(); err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	fmt.Fprint(w, "{}")
	return
}

func (i *jsonAPIHandler) DELETEModerator(w http.ResponseWriter, r *http.Request) {
	// If the moderator is already set tell them to use PUT
	modPath := path.Join(i.node.RepoPath, "root", "moderation")
	_, ferr := os.Stat(modPath)
	if os.IsNotExist(ferr) {
		ErrorResponse(w, http.StatusNotFound, "This node isn't set as a moderator")
		return
	}

	// Save self as moderator
	err := i.node.RemoveSelfAsModerator()
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Update followers/following
	err = i.node.UpdateFollow()
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Republish to IPNS
	if err := i.node.SeedNode(); err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	fmt.Fprintf(w, "{}")
	return
}

func (i *jsonAPIHandler) GETListing(w http.ResponseWriter, r *http.Request) {
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
		ErrorResponse(w, http.StatusNotFound, err.Error())
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
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	fmt.Fprint(w, string(out))
	return
}

func (i *jsonAPIHandler) GETFollowsMe(w http.ResponseWriter, r *http.Request) {
	_, peerId := path.Split(r.URL.Path)
	fmt.Fprintf(w, `{"followsMe": "%t"}`, i.node.Datastore.Followers().FollowsMe(peerId))
}

func (i *jsonAPIHandler) GETIsFollowing(w http.ResponseWriter, r *http.Request) {
	_, peerId := path.Split(r.URL.Path)
	fmt.Fprintf(w, `{"isFollowing": "%t"}`, i.node.Datastore.Following().IsFollowing(peerId))
}

func (i *jsonAPIHandler) POSTOrderConfirmation(w http.ResponseWriter, r *http.Request) {
	type orderConf struct {
		OrderId string `json:"orderId"`
		Reject  bool   `json:"reject"`
	}
	decoder := json.NewDecoder(r.Body)
	var conf orderConf
	err := decoder.Decode(&conf)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	contract, state, funded, _, err := i.node.Datastore.Sales().GetByOrderId(conf.OrderId)
	if err != nil {
		ErrorResponse(w, http.StatusNotFound, err.Error())
		return
	}
	if state != pb.OrderState_PENDING {
		ErrorResponse(w, http.StatusBadRequest, "order has already been confirmed")
		return
	}
	if !funded && !conf.Reject {
		ErrorResponse(w, http.StatusBadRequest, "payment address must be funded before confirmation")
		return
	}
	if !conf.Reject {
		err := i.node.ConfirmOfflineOrder(contract)
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
	} else {
		err := i.node.RejectOfflineOrder(contract)
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	fmt.Fprint(w, `{}`)
	return
}

func (i *jsonAPIHandler) POSTOrderCancel(w http.ResponseWriter, r *http.Request) {
	type orderCancel struct {
		OrderId string `json:"orderId"`
	}
	decoder := json.NewDecoder(r.Body)
	var can orderCancel
	err := decoder.Decode(&can)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	contract, state, _, _, err := i.node.Datastore.Purchases().GetByOrderId(can.OrderId)
	if err != nil {
		ErrorResponse(w, http.StatusNotFound, "order not found")
		return
	}
	if state != pb.OrderState_PENDING {
		ErrorResponse(w, http.StatusBadRequest, "order has already been confirmed")
		return
	}
	err = i.node.CancelOfflineOrder(contract)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	fmt.Fprint(w, `{}`)
	return
}

func (i *jsonAPIHandler) POSTResyncBlockchain(w http.ResponseWriter, r *http.Request) {
	i.node.Wallet.ReSyncBlockchain(0)
	fmt.Fprint(w, `{}`)
	return
}
