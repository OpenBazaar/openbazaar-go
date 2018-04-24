package api

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	mh "gx/ipfs/QmU9a9NV9RdPNwZQDYd5uKsm6N6LJLSvLbywDDYFbaaC6P/go-multihash"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"encoding/hex"

	"crypto/sha256"
	ps "gx/ipfs/QmPgDWmTmuzvP7QE5zwo1TmjbJme9pmZHNujB2453jkCTr/go-libp2p-peerstore"
	peer "gx/ipfs/QmXYjuNuxVzXKJCfWasQk1RqkhVLDM9jtUKhqc2WPQmFSB/go-libp2p-peer"
	"sync"

	"bytes"
	"gx/ipfs/QmNp85zy9RLrQ5oQD4hPyS39ezrrXpcaa7R4Y9kxdWQLLQ/go-cid"
	routing "gx/ipfs/QmUCS9EnqNq1kCnJds2eLDypBiS21aSiCf1MVzSUVB9TGA/go-libp2p-kad-dht"
	"io/ioutil"

	ds "gx/ipfs/QmVSase1JP7cq9QkPT46oNwdp9pT6kBkG3oqS14y3QcZjG/go-datastore"

	"github.com/OpenBazaar/jsonpb"
	"github.com/OpenBazaar/openbazaar-go/api/notifications"
	"github.com/OpenBazaar/openbazaar-go/core"
	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/spvwallet"
	"github.com/OpenBazaar/wallet-interface"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcutil/base58"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/ipfs/go-ipfs/core/coreunix"
	ipnspb "github.com/ipfs/go-ipfs/namesys/pb"
	ipnspath "github.com/ipfs/go-ipfs/path"
	lockfile "github.com/ipfs/go-ipfs/repo/fsrepo/lock"
)

type JsonAPIConfig struct {
	Headers       map[string]interface{}
	Enabled       bool
	Cors          *string
	Authenticated bool
	AllowedIPs    map[string]bool
	Cookie        http.Cookie
	Username      string
	Password      string
}

type jsonAPIHandler struct {
	config JsonAPIConfig
	node   *core.OpenBazaarNode
}

func newJsonAPIHandler(node *core.OpenBazaarNode, authCookie http.Cookie, config repo.APIConfig) (*jsonAPIHandler, error) {
	allowedIPs := make(map[string]bool)
	for _, ip := range config.AllowedIPs {
		allowedIPs[ip] = true
	}
	i := &jsonAPIHandler{
		config: JsonAPIConfig{
			Enabled:       config.Enabled,
			Cors:          config.CORS,
			Headers:       config.HTTPHeaders,
			Authenticated: config.Authenticated,
			AllowedIPs:    allowedIPs,
			Cookie:        authCookie,
			Username:      config.Username,
			Password:      config.Password,
		},
		node: node,
	}
	return i, nil
}

func (i *jsonAPIHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	u, err := url.Parse(r.URL.Path)
	if err != nil {
		log.Error(err)
		return
	}
	if !i.config.Enabled && !gatewayAllowedPath(u.Path, r.Method) {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, "403 - Forbidden")
		return
	}
	if len(i.config.AllowedIPs) > 0 {
		remoteAddr := strings.Split(r.RemoteAddr, ":")
		if !i.config.AllowedIPs[remoteAddr[0]] {
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprint(w, "403 - Forbidden")
			return
		}
	}

	if i.config.Cors != nil {
		w.Header().Set("Access-Control-Allow-Origin", *i.config.Cors)
		w.Header().Set("Access-Control-Allow-Methods", "PUT,POST,DELETE,GET,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
	}

	for k, v := range i.config.Headers {
		w.Header()[k] = v.([]string)
	}

	if i.config.Authenticated {
		if i.config.Username == "" || i.config.Password == "" {
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
		} else {

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				fmt.Fprint(w, "200 - OK")
				return
			}

			username, password, ok := r.BasicAuth()
			h := sha256.Sum256([]byte(password))
			password = hex.EncodeToString(h[:])
			if !ok || username != i.config.Username || strings.ToLower(password) != strings.ToLower(i.config.Password) {
				w.WriteHeader(http.StatusForbidden)
				fmt.Fprint(w, "403 - Forbidden")
				return
			}
		}
	}

	// Stop here if its Preflighted OPTIONS request
	if r.Method == "OPTIONS" {
		return
	}
	r.Header.Del("Cookie")
	r.Header.Del("Authorization")
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

	w.Header().Add("Content-Type", "application/json")
	switch r.Method {
	case "GET":
		get(i, u.String(), w, r)
	case "POST":
		post(i, u.String(), w, r)
	case "PUT":
		put(i, u.String(), w, r)
	case "DELETE":
		deleter(i, u.String(), w, r)
	case "PATCH":
		patch(i, u.String(), w, r)
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

func SanitizedResponse(w http.ResponseWriter, response string) {
	ret, err := SanitizeJSON([]byte(response))
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	fmt.Fprint(w, string(ret))
}

func SanitizedResponseM(w http.ResponseWriter, response string, m proto.Message) {
	out, err := SanitizeProtobuf(response, m)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	fmt.Fprint(w, string(out))
}

func (i *jsonAPIHandler) POSTProfile(w http.ResponseWriter, r *http.Request) {

	// If the profile is already set tell them to use PUT
	profilePath := path.Join(i.node.RepoPath, "root", "profile.json")
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

	// Maybe set as moderator
	if profile.Moderator {
		if err := i.node.SetSelfAsModerator(profile.ModeratorInfo); err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
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

	// Write profile back out as JSON
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
	SanitizedResponseM(w, out, new(pb.Profile))
	return
}

func (i *jsonAPIHandler) PUTProfile(w http.ResponseWriter, r *http.Request) {

	// If profile is not set tell them to use POST
	currentProfile, err := i.node.GetProfile()
	if err != nil {
		ErrorResponse(w, http.StatusNotFound, "Profile doesn't exist yet. Use POST.")
		return
	}

	// Check JSON decoding and add proper indentation
	profile := new(pb.Profile)
	err = jsonpb.Unmarshal(r.Body, profile)
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

	// Update moderator
	if profile.Moderator && currentProfile.Moderator != profile.Moderator {
		if err := i.node.SetSelfAsModerator(nil); err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
	} else if !profile.Moderator && currentProfile.Moderator != profile.Moderator {
		if err := i.node.RemoveSelfAsModerator(); err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
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

	// Return the profile in JSON format
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
	SanitizedResponseM(w, out, new(pb.Profile))
	return
}

func (i *jsonAPIHandler) PATCHProfile(w http.ResponseWriter, r *http.Request) {
	// If profile is not set tell them to use POST
	profilePath := path.Join(i.node.RepoPath, "root", "profile.json")
	_, ferr := os.Stat(profilePath)
	if os.IsNotExist(ferr) {
		ErrorResponse(w, http.StatusNotFound, "Profile doesn't exist yet. Use POST.")
		return
	}

	// Read json data into interface
	d := json.NewDecoder(r.Body)
	d.UseNumber()

	var patch interface{}
	err := d.Decode(&patch)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	// Apply patch
	err = i.node.PatchProfile(patch.(map[string]interface{}))
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

	SanitizedResponse(w, `{}`)
	return
}

func (i *jsonAPIHandler) POSTAvatar(w http.ResponseWriter, r *http.Request) {
	type ImgData struct {
		Avatar string `json:"avatar"`
	}
	decoder := json.NewDecoder(r.Body)
	data := new(ImgData)
	err := decoder.Decode(&data)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	hashes, err := i.node.SetAvatarImages(data.Avatar)
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
	jsonHashes, err := json.MarshalIndent(hashes, "", "    ")
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	SanitizedResponse(w, string(jsonHashes))
	return
}

func (i *jsonAPIHandler) POSTHeader(w http.ResponseWriter, r *http.Request) {
	type ImgData struct {
		Header string `json:"header"`
	}
	decoder := json.NewDecoder(r.Body)
	data := new(ImgData)
	err := decoder.Decode(&data)

	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	hashes, err := i.node.SetHeaderImages(data.Header)
	if err != nil {
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
	jsonHashes, err := json.MarshalIndent(hashes, "", "    ")
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	SanitizedResponse(w, string(jsonHashes))
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
		Filename string           `json:"filename"`
		Hashes   pb.Profile_Image `json:"hashes"`
	}
	var retData []retImage
	for _, img := range images {
		hashes, err := i.node.SetProductImages(img.Image, img.Filename)
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		rtimg := retImage{img.Filename, *hashes}
		retData = append(retData, rtimg)
	}
	jsonHashes, err := json.MarshalIndent(retData, "", "    ")
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	SanitizedResponse(w, string(jsonHashes))
	return
}

func (i *jsonAPIHandler) POSTListing(w http.ResponseWriter, r *http.Request) {
	ld := new(pb.Listing)
	err := jsonpb.Unmarshal(r.Body, ld)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	err = i.node.CreateListing(ld)
	if err != nil {
		if err == core.ErrListingAlreadyExists {
			ErrorResponse(w, http.StatusConflict, "Listing already exists. Use PUT.")
			return
		}

		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	SanitizedResponse(w, fmt.Sprintf(`{"slug": "%s"}`, ld.Slug))
}

func (i *jsonAPIHandler) PUTListing(w http.ResponseWriter, r *http.Request) {
	ld := new(pb.Listing)
	err := jsonpb.Unmarshal(r.Body, ld)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	err = i.node.UpdateListing(ld)
	if err != nil {
		if err == core.ErrListingDoesNotExist {
			ErrorResponse(w, http.StatusNotFound, "Listing not found.")
			return
		}

		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	SanitizedResponse(w, `{}`)
	return
}

func (i *jsonAPIHandler) DELETEListing(w http.ResponseWriter, r *http.Request) {
	_, slug := path.Split(r.URL.Path)
	listingPath := path.Join(i.node.RepoPath, "root", "listings", slug+".json")
	_, ferr := os.Stat(listingPath)
	if os.IsNotExist(ferr) {
		ErrorResponse(w, http.StatusNotFound, "Listing not found.")
		return
	}
	err := i.node.DeleteListing(slug)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	err = i.node.UpdateFollow()
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, "File Write Error: "+err.Error())
		return
	}
	if err := i.node.SeedNode(); err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	SanitizedResponse(w, `{}`)
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
		PaymentAddress string `json:"paymentAddress"`
		Amount         uint64 `json:"amount"`
		VendorOnline   bool   `json:"vendorOnline"`
		OrderId        string `json:"orderId"`
	}
	ret := purchaseReturn{paymentAddr, amount, online, orderId}
	b, err := json.MarshalIndent(ret, "", "    ")
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	SanitizedResponse(w, string(b))
	return
}

func (i *jsonAPIHandler) GETStatus(w http.ResponseWriter, r *http.Request) {
	_, peerId := path.Split(r.URL.Path)
	status, err := i.node.GetPeerStatus(peerId)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	SanitizedResponse(w, fmt.Sprintf(`{"status": "%s"}`, status))
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
	SanitizedResponse(w, string(peerJson))
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
	SanitizedResponse(w, `{}`)
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
	SanitizedResponse(w, `{}`)
	return
}

func (i *jsonAPIHandler) GETAddress(w http.ResponseWriter, r *http.Request) {
	addr := i.node.Wallet.CurrentAddress(wallet.EXTERNAL)
	SanitizedResponse(w, fmt.Sprintf(`{"address": "%s"}`, addr.EncodeAddress()))
}

func (i *jsonAPIHandler) GETMnemonic(w http.ResponseWriter, r *http.Request) {
	mn, err := i.node.Datastore.Config().GetMnemonic()
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	SanitizedResponse(w, fmt.Sprintf(`{"mnemonic": "%s"}`, mn))
}

func (i *jsonAPIHandler) GETBalance(w http.ResponseWriter, r *http.Request) {
	/*_, ok := i.node.Wallet.(*bitcoind.BitcoindWallet)
	if ok {
		select {
		case <-i.node.Wallet.(*bitcoind.BitcoindWallet).InitChan():
			break
		default:
			ErrorResponse(w, http.StatusServiceUnavailable, "ERROR_WALLET_UNINITIALIZED")
			return
		}
	}
	_, ok = i.node.Wallet.(*zcashd.ZcashdWallet)
	if ok {
		select {
		case <-i.node.Wallet.(*zcashd.ZcashdWallet).InitChan():
			break
		default:
			ErrorResponse(w, http.StatusServiceUnavailable, "ERROR_WALLET_UNINITIALIZED")
			return
		}
	}*/
	confirmed, unconfirmed := i.node.Wallet.Balance()
	SanitizedResponse(w, fmt.Sprintf(`{"confirmed": %d, "unconfirmed": %d}`, int(confirmed), int(unconfirmed)))
}

func (i *jsonAPIHandler) POSTSpendCoins(w http.ResponseWriter, r *http.Request) {
	type Send struct {
		Address  string `json:"address"`
		Amount   int64  `json:"amount"`
		FeeLevel string `json:"feeLevel"`
		Memo     string `json:"memo"`
	}
	decoder := json.NewDecoder(r.Body)
	var snd Send
	err := decoder.Decode(&snd)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	var feeLevel wallet.FeeLevel
	switch strings.ToUpper(snd.FeeLevel) {
	case "PRIORITY":
		feeLevel = wallet.PRIOIRTY
	case "NORMAL":
		feeLevel = wallet.NORMAL
	case "ECONOMIC":
		feeLevel = wallet.ECONOMIC
	default:
		feeLevel = wallet.NORMAL
	}
	addr, err := i.node.Wallet.DecodeAddress(snd.Address)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, "ERROR_INVALID_ADDRESS")
		return
	}
	txid, err := i.node.Wallet.Spend(snd.Amount, addr, feeLevel)
	if err != nil {
		switch {
		case err == wallet.ErrorInsuffientFunds:
			ErrorResponse(w, http.StatusBadRequest, `ERROR_INSUFFICIENT_FUNDS`)
			return
		case err == wallet.ErrorDustAmount:
			ErrorResponse(w, http.StatusBadRequest, `ERROR_DUST_AMOUNT`)
			return
		default:
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	var orderId string
	var thumbnail string
	var memo string
	var title string
	contract, _, _, _, err := i.node.Datastore.Purchases().GetByPaymentAddress(addr)
	if contract != nil && err == nil {
		orderId, _ = i.node.CalcOrderId(contract.BuyerOrder)
		if contract.VendorListings[0].Item != nil && len(contract.VendorListings[0].Item.Images) > 0 {
			thumbnail = contract.VendorListings[0].Item.Images[0].Tiny
			title = contract.VendorListings[0].Item.Title
		}
	}
	if title == "" {
		memo = snd.Memo
	} else {
		memo = title
	}

	if err := i.node.Datastore.TxMetadata().Put(repo.Metadata{
		Txid:       txid.String(),
		Address:    snd.Address,
		Memo:       memo,
		OrderId:    orderId,
		Thumbnail:  thumbnail,
		CanBumpFee: false,
	}); err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	type response struct {
		Txid               string    `json:"txid"`
		Amount             int64     `json:"amount"`
		ConfirmedBalance   int64     `json:"confirmedBalance"`
		UnconfirmedBalance int64     `json:"unconfirmedBalance"`
		Timestamp          time.Time `json:"timestamp"`
		Memo               string    `json:"memo"`
	}
	confirmed, unconfirmed := i.node.Wallet.Balance()
	txn, err := i.node.Wallet.GetTransaction(*txid)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	resp := &response{
		Txid:               txid.String(),
		ConfirmedBalance:   confirmed,
		UnconfirmedBalance: unconfirmed,
		Amount:             -(txn.Value),
		Timestamp:          txn.Timestamp,
		Memo:               memo,
	}
	ser, err := json.MarshalIndent(resp, "", "    ")
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	SanitizedResponse(w, string(ser))
	return
}

func (i *jsonAPIHandler) GETConfig(w http.ResponseWriter, r *http.Request) {
	var usingTor bool
	if i.node.TorDialer != nil {
		usingTor = true
	}
	c := struct {
		PeerId         string `json:"peerID"`
		CryptoCurrency string `json:"cryptoCurrency"`
		Testnet        bool   `json:"testnet"`
		Tor            bool   `json:"tor"`
	}{
		PeerId:         i.node.IPFSIdentityString(),
		CryptoCurrency: core.NormalizeCurrencyCode(i.node.Wallet.CurrencyCode()),
		Testnet:        i.node.TestNetworkEnabled(),
		Tor:            usingTor,
	}
	ser, err := json.MarshalIndent(c, "", "    ")
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	SanitizedResponse(w, string(ser))
}

func (i *jsonAPIHandler) POSTSettings(w http.ResponseWriter, r *http.Request) {
	var settings repo.SettingsData
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&settings)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	if err = validateSMTPSettings(settings); err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	_, err = i.node.Datastore.Settings().Get()
	if err == nil {
		ErrorResponse(w, http.StatusConflict, "Settings is already set. Use PUT.")
		return
	}

	if settings.MisPaymentBuffer == nil {
		i := float32(1)
		settings.MisPaymentBuffer = &i
	}
	if settings.BlockedNodes != nil {
		var blockedIds []peer.ID
		for _, pid := range *settings.BlockedNodes {
			id, err := peer.IDB58Decode(pid)
			if err != nil {
				continue
			}
			blockedIds = append(blockedIds, id)
		}
		i.node.BanManager.SetBlockedIds(blockedIds)
	}
	if settings.StoreModerators != nil {
		go i.node.NotifyModerators(*settings.StoreModerators)
		if err := i.node.SetModeratorsOnListings(*settings.StoreModerators); err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
		}
		if err := i.node.SeedNode(); err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
		}
	}
	err = i.node.Datastore.Settings().Put(settings)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	settings.Version = &i.node.UserAgent
	ser, err := json.MarshalIndent(settings, "", "    ")
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	SanitizedResponse(w, string(ser))
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
	if err = validateSMTPSettings(settings); err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	_, err = i.node.Datastore.Settings().Get()
	if err != nil {
		ErrorResponse(w, http.StatusNotFound, "Settings is not yet set. Use POST.")
		return
	}
	if settings.BlockedNodes != nil {
		var blockedIds []peer.ID
		for _, pid := range *settings.BlockedNodes {
			id, err := peer.IDB58Decode(pid)
			if err != nil {
				continue
			}
			blockedIds = append(blockedIds, id)
		}
		i.node.BanManager.SetBlockedIds(blockedIds)
	}
	if settings.StoreModerators != nil {
		go i.node.NotifyModerators(*settings.StoreModerators)
		if err := i.node.SetModeratorsOnListings(*settings.StoreModerators); err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
		}
		if err := i.node.SeedNode(); err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
		}
	}

	err = i.node.Datastore.Settings().Put(settings)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	SanitizedResponse(w, `{}`)
	return
}

func (i *jsonAPIHandler) GETSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := i.node.Datastore.Settings().Get()
	if err != nil {
		ErrorResponse(w, http.StatusNotFound, err.Error())
		return
	}
	settings.Version = &i.node.UserAgent
	settingsJson, err := json.MarshalIndent(&settings, "", "    ")
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	SanitizedResponse(w, string(settingsJson))
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
	if err = validateSMTPSettings(settings); err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	if settings.StoreModerators != nil {
		go i.node.NotifyModerators(*settings.StoreModerators)
		if err := i.node.SetModeratorsOnListings(*settings.StoreModerators); err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
		}
		if err := i.node.SeedNode(); err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
		}
	}
	if settings.BlockedNodes != nil {
		var blockedIds []peer.ID
		for _, pid := range *settings.BlockedNodes {
			id, err := peer.IDB58Decode(pid)
			if err != nil {
				continue
			}
			blockedIds = append(blockedIds, id)
		}
		i.node.BanManager.SetBlockedIds(blockedIds)
	}
	err = i.node.Datastore.Settings().Update(settings)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	SanitizedResponse(w, `{}`)
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
	SanitizedResponse(w, string(ret))
}

func (i *jsonAPIHandler) GETExchangeRate(w http.ResponseWriter, r *http.Request) {
	_, currencyCode := path.Split(r.URL.Path)
	if currencyCode == "" || strings.ToLower(currencyCode) == "exchangerate" {
		currencyMap, err := i.node.ExchangeRates.GetAllRates(true)
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		exchangeRateJson, err := json.MarshalIndent(currencyMap, "", "    ")
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		SanitizedResponse(w, string(exchangeRateJson))

	} else {
		rate, err := i.node.ExchangeRates.GetExchangeRate(currencyCode)
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		fmt.Fprintf(w, `%.2f`, rate)
	}
}

func (i *jsonAPIHandler) GETFollowers(w http.ResponseWriter, r *http.Request) {
	_, peerId := path.Split(r.URL.Path)
	if peerId == "" || strings.ToLower(peerId) == "followers" || peerId == i.node.IPFSIdentityString() {
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
		var followList []string
		for _, f := range followers {
			followList = append(followList, f.PeerId)
		}
		ret, err := json.MarshalIndent(followList, "", "    ")
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		if string(ret) == "null" {
			ret = []byte("[]")
		}
		SanitizedResponse(w, string(ret))
	} else {
		pid, err := i.node.NameSystem.Resolve(context.Background(), peerId)
		if err != nil {
			ErrorResponse(w, http.StatusNotFound, err.Error())
			return
		}
		peerId = pid.Pretty()
		followBytes, err := i.node.IPNSResolveThenCat(ipnspath.FromString(path.Join(peerId, "followers.json")), time.Minute)
		if err != nil {
			ErrorResponse(w, http.StatusNotFound, err.Error())
			return
		}
		var followers []repo.Follower
		err = json.Unmarshal(followBytes, &followers)
		if err != nil {
			ErrorResponse(w, http.StatusNotFound, err.Error())
			return
		}
		var followList []string
		for _, f := range followers {
			followList = append(followList, f.PeerId)
		}
		ret, err := json.MarshalIndent(followList, "", "    ")
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		if string(ret) == "null" {
			ret = []byte("[]")
		}
		w.Header().Set("Cache-Control", "public, max-age=600, immutable")
		SanitizedResponse(w, string(ret))
	}
}

func (i *jsonAPIHandler) GETFollowing(w http.ResponseWriter, r *http.Request) {
	_, peerId := path.Split(r.URL.Path)
	if peerId == "" || strings.ToLower(peerId) == "following" || peerId == i.node.IPFSIdentityString() {
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
		followers, err := i.node.Datastore.Following().Get(offset, int(l))
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		ret, _ := json.MarshalIndent(followers, "", "    ")
		if string(ret) == "null" {
			ret = []byte("[]")
		}
		SanitizedResponse(w, string(ret))
	} else {
		pid, err := i.node.NameSystem.Resolve(context.Background(), peerId)
		if err != nil {
			ErrorResponse(w, http.StatusNotFound, err.Error())
			return
		}
		peerId = pid.Pretty()
		followBytes, err := i.node.IPNSResolveThenCat(ipnspath.FromString(path.Join(peerId, "following.json")), time.Minute)
		if err != nil {
			ErrorResponse(w, http.StatusNotFound, err.Error())
			return
		}
		w.Header().Set("Cache-Control", "public, max-age=600, immutable")
		SanitizedResponse(w, string(followBytes))
	}
}

func (i *jsonAPIHandler) GETInventory(w http.ResponseWriter, r *http.Request) {
	// Get optional peerID and slug parameters
	var (
		peerIDString string
		slug         string
	)

	parts := strings.Split(r.URL.Path, "/")
	if len(parts) > 3 {
		peerIDString = parts[3]
	}
	if len(parts) > 4 {
		slug = parts[4]
	}

	// If we want our own inventory get it from the local database and return
	getPersonalInventory := (peerIDString == "" || peerIDString == i.node.IPFSIdentityString())
	if getPersonalInventory {
		type inv struct {
			Slug     string `json:"slug"`
			Variant  int    `json:"variant"`
			Quantity int    `json:"quantity"`
		}
		var invList []inv
		inventory, err := i.node.Datastore.Inventory().GetAll()
		if err != nil {
			fmt.Fprint(w, `[]`)
			return
		}
		for itemSlug, m := range inventory {
			for variant, count := range m {
				if slug != "" && slug != itemSlug {
					continue
				}

				i := inv{itemSlug, variant, count}
				invList = append(invList, i)
			}
		}
		ret, _ := json.MarshalIndent(invList, "", "    ")
		if string(ret) == "null" {
			fmt.Fprint(w, `[]`)
			return
		}
		SanitizedResponse(w, string(ret))
		return
	}

	// If we want another peer's inventory crawl IPFS with an optional cache
	var err error
	useCacheBool := false
	useCacheString := r.URL.Query().Get("useCache")
	if len(useCacheString) > 0 {
		useCacheBool, err = strconv.ParseBool(useCacheString)
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	peerID, err := peer.IDB58Decode(peerIDString)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	if slug == "" {
		inventoryBytes, err := i.node.GetPublishedInventoryBytes(peerID, useCacheBool)
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		SanitizedResponse(w, string(inventoryBytes))
		return
	}

	inventoryBytes, err := i.node.GetPublishedInventoryBytesForSlug(peerID, slug, useCacheBool)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	SanitizedResponse(w, string(inventoryBytes))
}

func (i *jsonAPIHandler) POSTInventory(w http.ResponseWriter, r *http.Request) {
	type inv struct {
		Slug     string `json:"slug"`
		Variant  int    `json:"variant"`
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
		err = i.node.Datastore.Inventory().Put(in.Slug, in.Variant, in.Quantity)
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	err = i.node.PublishInventory()
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	SanitizedResponse(w, `{}`)
	return
}

func (i *jsonAPIHandler) PUTModerator(w http.ResponseWriter, r *http.Request) {
	profilePath := path.Join(i.node.RepoPath, "root", "profile.json")
	_, ferr := os.Stat(profilePath)
	if os.IsNotExist(ferr) {
		ErrorResponse(w, http.StatusConflict, "Profile does not exist. Create one first.")
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
		ErrorResponse(w, http.StatusInternalServerError, "File Write Error: "+err.Error())
		return
	}

	// Republish to IPNS
	if err := i.node.SeedNode(); err != nil {
		ErrorResponse(w, http.StatusInternalServerError, "IPNS Error: "+err.Error())
		return
	}
	SanitizedResponse(w, "{}")
	return
}

func (i *jsonAPIHandler) DELETEModerator(w http.ResponseWriter, r *http.Request) {
	profile, err := i.node.GetProfile()
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	profile.Moderator = false
	profile.ModeratorInfo = nil
	err = i.node.UpdateProfile(&profile)
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
	SanitizedResponse(w, "{}")
	return
}

func (i *jsonAPIHandler) GETListings(w http.ResponseWriter, r *http.Request) {
	_, peerId := path.Split(r.URL.Path)
	if peerId == "" || strings.ToLower(peerId) == "listings" || peerId == i.node.IPFSIdentityString() {
		listingsBytes, err := i.node.GetListings()
		if err != nil {
			ErrorResponse(w, http.StatusNotFound, err.Error())
			return
		}
		SanitizedResponse(w, string(listingsBytes))
	} else {
		pid, err := i.node.NameSystem.Resolve(context.Background(), peerId)
		if err != nil {
			ErrorResponse(w, http.StatusNotFound, err.Error())
			return
		}
		peerId = pid.Pretty()
		listingsBytes, err := i.node.IPNSResolveThenCat(ipnspath.FromString(path.Join(peerId, "listings.json")), time.Minute)
		if err != nil {
			ErrorResponse(w, http.StatusNotFound, err.Error())
			return
		}
		SanitizedResponse(w, string(listingsBytes))
		w.Header().Set("Cache-Control", "public, max-age=600, immutable")
	}
}

func (i *jsonAPIHandler) GETListing(w http.ResponseWriter, r *http.Request) {
	urlPath, listingId := path.Split(r.URL.Path)
	_, peerId := path.Split(urlPath[:len(urlPath)-1])
	m := jsonpb.Marshaler{
		EnumsAsInts:  false,
		EmitDefaults: false,
		Indent:       "    ",
		OrigName:     false,
	}
	if peerId == "" || strings.ToLower(peerId) == "listing" || peerId == i.node.IPFSIdentityString() {
		sl := new(pb.SignedListing)
		_, err := cid.Decode(listingId)
		if err == nil {
			sl, err = i.node.GetListingFromHash(listingId)
			if err != nil {
				ErrorResponse(w, http.StatusNotFound, "Listing not found.")
				return
			}
			sl.Hash = listingId
		} else {
			sl, err = i.node.GetListingFromSlug(listingId)
			if err != nil {
				ErrorResponse(w, http.StatusNotFound, "Listing not found.")
				return
			}
			hash, err := ipfs.GetHashOfFile(i.node.Context, path.Join(i.node.RepoPath, "root", "listings", listingId+".json"))
			if err != nil {
				ErrorResponse(w, http.StatusInternalServerError, err.Error())
				return
			}
			sl.Hash = hash
		}
		savedCoupons, err := i.node.Datastore.Coupons().Get(sl.Listing.Slug)
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		for _, coupon := range sl.Listing.Coupons {
			for _, c := range savedCoupons {
				if coupon.GetHash() == c.Hash {
					coupon.Code = &pb.Listing_Coupon_DiscountCode{c.Code}
					break
				}
			}
		}
		if sl.Listing.Metadata != nil && sl.Listing.Metadata.Version == 1 {
			for _, so := range sl.Listing.ShippingOptions {
				for _, ser := range so.Services {
					ser.AdditionalItemPrice = ser.Price
				}
			}
		}

		out, err := m.MarshalToString(sl)
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		SanitizedResponseM(w, string(out), new(pb.SignedListing))
		return
	} else {
		var listingBytes []byte
		var hash string
		_, err := cid.Decode(listingId)
		if err == nil {
			listingBytes, err = ipfs.Cat(i.node.Context, listingId, time.Minute)
			if err != nil {
				ErrorResponse(w, http.StatusNotFound, err.Error())
				return
			}
			hash = listingId
			w.Header().Set("Cache-Control", "public, max-age=29030400, immutable")
		} else {
			pid, err := i.node.NameSystem.Resolve(context.Background(), peerId)
			if err != nil {
				ErrorResponse(w, http.StatusNotFound, err.Error())
				return
			}
			peerId = pid.Pretty()
			listingBytes, err = i.node.IPNSResolveThenCat(ipnspath.FromString(path.Join(peerId, "listings", listingId+".json")), time.Minute)
			if err != nil {
				ErrorResponse(w, http.StatusNotFound, err.Error())
				return
			}
			hash, err = ipfs.GetHash(i.node.Context, bytes.NewReader(listingBytes))
			if err != nil {
				ErrorResponse(w, http.StatusInternalServerError, err.Error())
				return
			}
			w.Header().Set("Cache-Control", "public, max-age=600, immutable")
		}
		sl := new(pb.SignedListing)
		err = jsonpb.UnmarshalString(string(listingBytes), sl)
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		sl.Hash = hash
		out, err := m.MarshalToString(sl)
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		SanitizedResponseM(w, out, new(pb.SignedListing))
	}
}

func (i *jsonAPIHandler) GETProfile(w http.ResponseWriter, r *http.Request) {
	_, peerId := path.Split(r.URL.Path)
	var profile pb.Profile
	var err error
	cacheBool := r.URL.Query().Get("usecache")
	useCache, _ := strconv.ParseBool(cacheBool)

	if peerId == "" || strings.ToLower(peerId) == "profile" || peerId == i.node.IPFSIdentityString() {
		profile, err = i.node.GetProfile()
		if err != nil && err == core.ErrorProfileNotFound {
			ErrorResponse(w, http.StatusNotFound, err.Error())
			return
		} else if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
	} else {
		pid, err := i.node.NameSystem.Resolve(context.Background(), peerId)
		if err != nil {
			ErrorResponse(w, http.StatusNotFound, err.Error())
			return
		}
		peerId = pid.Pretty()
		profile, err = i.node.FetchProfile(peerId, useCache)
		if err != nil {
			ErrorResponse(w, http.StatusNotFound, err.Error())
			return
		}
		if profile.PeerID != peerId {
			ErrorResponse(w, http.StatusNotFound, err.Error())
			return
		}
		w.Header().Set("Cache-Control", "public, max-age=600, immutable")
	}
	m := jsonpb.Marshaler{
		EnumsAsInts:  false,
		EmitDefaults: true,
		Indent:       "    ",
		OrigName:     false,
	}
	out, err := m.MarshalToString(&profile)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	SanitizedResponseM(w, out, new(pb.Profile))
}

func (i *jsonAPIHandler) GETFollowsMe(w http.ResponseWriter, r *http.Request) {
	_, peerId := path.Split(r.URL.Path)
	SanitizedResponse(w, fmt.Sprintf(`{"followsMe": %t}`, i.node.Datastore.Followers().FollowsMe(peerId)))
}

func (i *jsonAPIHandler) GETIsFollowing(w http.ResponseWriter, r *http.Request) {
	_, peerId := path.Split(r.URL.Path)
	SanitizedResponse(w, fmt.Sprintf(`{"isFollowing": %t}`, i.node.Datastore.Following().IsFollowing(peerId)))
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
	contract, state, funded, records, _, err := i.node.Datastore.Sales().GetByOrderId(conf.OrderId)
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
		err := i.node.ConfirmOfflineOrder(contract, records)
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
	} else {
		err := i.node.RejectOfflineOrder(contract, records)
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	SanitizedResponse(w, `{}`)
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
	contract, state, _, records, _, err := i.node.Datastore.Purchases().GetByOrderId(can.OrderId)
	if err != nil {
		ErrorResponse(w, http.StatusNotFound, "order not found")
		return
	}

	if !((state == pb.OrderState_PENDING || state == pb.OrderState_PROCESSING_ERROR) && len(records) > 0) || !(state == pb.OrderState_PENDING || state == pb.OrderState_PROCESSING_ERROR) || contract.BuyerOrder.Payment.Method == pb.Order_Payment_MODERATED {
		ErrorResponse(w, http.StatusBadRequest, "order must be PENDING or PROCESSING_ERROR and only a direct payment to cancel")
		return
	}
	err = i.node.CancelOfflineOrder(contract, records)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	SanitizedResponse(w, `{}`)
	return
}

func (i *jsonAPIHandler) POSTResyncBlockchain(w http.ResponseWriter, r *http.Request) {
	creationDate, err := i.node.Datastore.Config().GetCreationDate()
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	i.node.Wallet.ReSyncBlockchain(creationDate)
	SanitizedResponse(w, `{}`)
	return
}

func (i *jsonAPIHandler) GETOrder(w http.ResponseWriter, r *http.Request) {
	_, orderId := path.Split(r.URL.Path)
	var err error
	var isSale bool
	var contract *pb.RicardianContract
	var state pb.OrderState
	var funded bool
	var records []*wallet.TransactionRecord
	var read bool
	contract, state, funded, records, read, err = i.node.Datastore.Purchases().GetByOrderId(orderId)
	if err != nil {
		contract, state, funded, records, read, err = i.node.Datastore.Sales().GetByOrderId(orderId)
		if err != nil {
			ErrorResponse(w, http.StatusNotFound, "Order not found")
			return
		}
		isSale = true
	}
	resp := new(pb.OrderRespApi)
	resp.Contract = contract
	resp.Funded = funded
	resp.Read = read
	resp.State = state

	paymentTxs, refundTx, err := i.node.BuildTransactionRecords(contract, records, state)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	resp.PaymentAddressTransactions = paymentTxs
	resp.RefundAddressTransaction = refundTx

	unread, err := i.node.Datastore.Chat().GetUnreadCount(orderId)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	resp.UnreadChatMessages = uint64(unread)

	m := jsonpb.Marshaler{
		EnumsAsInts:  false,
		EmitDefaults: true,
		Indent:       "    ",
		OrigName:     false,
	}
	out, err := m.MarshalToString(resp)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	if isSale {
		i.node.Datastore.Sales().MarkAsRead(orderId)
	} else {
		i.node.Datastore.Purchases().MarkAsRead(orderId)
	}
	SanitizedResponseM(w, out, new(pb.OrderRespApi))
}

func (i *jsonAPIHandler) POSTShutdown(w http.ResponseWriter, r *http.Request) {
	shutdown := func() {
		log.Info("OpenBazaar Server shutting down...")
		time.Sleep(time.Second)
		if core.Node != nil {
			core.Node.Datastore.Close()
			repoLockFile := filepath.Join(core.Node.RepoPath, lockfile.LockFile)
			os.Remove(repoLockFile)
			core.Node.Wallet.Close()
			core.Node.IpfsNode.Close()
		}
		os.Exit(1)
	}
	go shutdown()
	SanitizedResponse(w, `{}`)
	return
}

func (i *jsonAPIHandler) POSTRefund(w http.ResponseWriter, r *http.Request) {
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
	contract, state, _, records, _, err := i.node.Datastore.Sales().GetByOrderId(can.OrderId)
	if err != nil {
		ErrorResponse(w, http.StatusNotFound, "order not found")
		return
	}
	if state != pb.OrderState_AWAITING_FULFILLMENT && state != pb.OrderState_PARTIALLY_FULFILLED {
		ErrorResponse(w, http.StatusBadRequest, "order must be AWAITING_FULFILLMENT, or PARTIALLY_FULFILLED")
		return
	}
	err = i.node.RefundOrder(contract, records)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	SanitizedResponse(w, `{}`)
	return
}

func (i *jsonAPIHandler) GETModerators(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("async")
	async, _ := strconv.ParseBool(query)
	include := r.URL.Query().Get("include")

	ctx := context.Background()
	if !async {
		removeDuplicates := func(xs []string) {
			found := make(map[string]bool)
			j := 0
			for i, x := range xs {
				if !found[x] {
					found[x] = true
					(xs)[j] = (xs)[i]
					j++
				}
			}
			xs = (xs)[:j]
		}
		peerInfoList, err := ipfs.FindPointers(i.node.IpfsNode.Routing.(*routing.IpfsDHT), ctx, core.ModeratorPointerID, 64)
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		var mods []string
		for _, p := range peerInfoList {
			id, err := core.ExtractIDFromPointer(p)
			if err != nil {
				continue
			}
			mods = append(mods, id)
		}
		var resp string
		removeDuplicates(mods)
		if strings.ToLower(include) == "profile" {
			var withProfiles []string
			var wg sync.WaitGroup
			for _, mod := range mods {
				wg.Add(1)
				go func(m string) {
					profile, err := i.node.FetchProfile(m, false)
					if err != nil {
						wg.Done()
						return
					}
					resp := &pb.PeerAndProfile{m, &profile}
					mar := jsonpb.Marshaler{
						EnumsAsInts:  false,
						EmitDefaults: true,
						Indent:       "    ",
						OrigName:     false,
					}
					respJson, err := mar.MarshalToString(resp)
					if err != nil {
						return
					}
					withProfiles = append(withProfiles, respJson)
					wg.Done()
				}(mod)
			}
			wg.Wait()
			resp = "[\n"
			max := len(withProfiles)
			for i, r := range withProfiles {
				lines := strings.Split(r, "\n")
				maxx := len(lines)
				for x, s := range lines {
					resp += "    "
					resp += s
					if x != maxx-1 {
						resp += "\n"
					}
				}
				if i != max-1 {
					resp += ",\n"
				}
			}
			resp += "\n]"
		} else {
			res, err := json.MarshalIndent(mods, "", "    ")
			if err != nil {
				ErrorResponse(w, http.StatusInternalServerError, err.Error())
				return
			}
			resp = string(res)
		}
		if resp == "null" {
			resp = "[]"
		}
		SanitizedResponse(w, resp)
	} else {
		idBytes := make([]byte, 16)
		rand.Read(idBytes)
		id := base58.Encode(idBytes)

		type resp struct {
			Id string `json:"id"`
		}
		response := resp{id}
		respJson, _ := json.MarshalIndent(response, "", "    ")
		w.WriteHeader(http.StatusAccepted)
		SanitizedResponse(w, string(respJson))
		go func() {
			type wsResp struct {
				Id     string `json:"id"`
				PeerId string `json:"peerId"`
			}
			peerChan := ipfs.FindPointersAsync(i.node.IpfsNode.Routing.(*routing.IpfsDHT), ctx, core.ModeratorPointerID, 64)

			found := make(map[string]bool)
			foundMu := sync.Mutex{}
			for p := range peerChan {
				go func(pi ps.PeerInfo) {
					pid, err := core.ExtractIDFromPointer(pi)
					if err != nil {
						return
					}

					// Check and set the peer in `found` with locking
					foundMu.Lock()
					if found[pid] {
						foundMu.Unlock()
						return
					}
					found[pid] = true
					foundMu.Unlock()

					if strings.ToLower(include) == "profile" {
						profile, err := i.node.FetchProfile(pid, false)
						if err != nil {
							return
						}
						resp := pb.PeerAndProfileWithID{id, pid, &profile}
						m := jsonpb.Marshaler{
							EnumsAsInts:  false,
							EmitDefaults: true,
							Indent:       "    ",
							OrigName:     false,
						}
						respJson, err := m.MarshalToString(&resp)
						if err != nil {
							return
						}
						b, err := SanitizeProtobuf(respJson, new(pb.PeerAndProfileWithID))
						if err != nil {
							return
						}
						i.node.Broadcast <- b
					} else {
						resp := wsResp{id, pid}
						respJson, err := json.MarshalIndent(resp, "", "    ")
						if err != nil {
							return
						}
						i.node.Broadcast <- []byte(respJson)
					}
				}(p)
			}
		}()
	}
}

func (i *jsonAPIHandler) POSTOrderFulfill(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	var fulfill pb.OrderFulfillment
	err := decoder.Decode(&fulfill)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	contract, state, _, records, _, err := i.node.Datastore.Sales().GetByOrderId(fulfill.OrderId)
	if err != nil {
		ErrorResponse(w, http.StatusNotFound, "order not found")
		return
	}
	if state != pb.OrderState_AWAITING_FULFILLMENT && state != pb.OrderState_PARTIALLY_FULFILLED {
		ErrorResponse(w, http.StatusBadRequest, "order must be in state AWAITING_FULFILLMENT or PARTIALLY_FULFILLED to fulfill")
		return
	}
	err = i.node.FulfillOrder(&fulfill, contract, records)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	SanitizedResponse(w, `{}`)
	return
}

func (i *jsonAPIHandler) POSTOrderComplete(w http.ResponseWriter, r *http.Request) {
	checkRatingValue := func(val int) bool {
		if val < core.RatingMin || val > core.RatingMax {
			ErrorResponse(w, http.StatusBadRequest, "rating values must be between 1 and 5")
			return false
		}
		return true
	}
	decoder := json.NewDecoder(r.Body)
	var or core.OrderRatings
	err := decoder.Decode(&or)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	contract, state, _, records, _, err := i.node.Datastore.Purchases().GetByOrderId(or.OrderId)
	if err != nil {
		ErrorResponse(w, http.StatusNotFound, "order not found")
		return
	}
	for _, rd := range or.Ratings {
		if rd.Slug == "" {
			ErrorResponse(w, http.StatusBadRequest, "rating must contain the slug")
			return
		}
		if !checkRatingValue(rd.Overall) {
			return
		}
		if !checkRatingValue(rd.Quality) {
			return
		}
		if !checkRatingValue(rd.Description) {
			return
		}
		if !checkRatingValue(rd.DeliverySpeed) {
			return
		}
		if !checkRatingValue(rd.CustomerService) {
			return
		}
		if len(rd.Review) > core.ReviewMaxCharacters {
			ErrorResponse(w, http.StatusBadRequest, "too many characters in review")
			return
		}
	}

	if state != pb.OrderState_FULFILLED && state != pb.OrderState_RESOLVED {
		ErrorResponse(w, http.StatusBadRequest, "order must be either fulfilled or in closed dispute state to leave the rating")
		return
	}
	err = i.node.CompleteOrder(&or, contract, records)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	SanitizedResponse(w, `{}`)
	return
}

func (i *jsonAPIHandler) POSTOpenDispute(w http.ResponseWriter, r *http.Request) {
	type dispute struct {
		OrderID string `json:"orderId"`
		Claim   string `json:"claim"`
	}
	decoder := json.NewDecoder(r.Body)
	var d dispute
	err := decoder.Decode(&d)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	var isSale bool
	var contract *pb.RicardianContract
	var state pb.OrderState
	var records []*wallet.TransactionRecord
	contract, state, _, records, _, err = i.node.Datastore.Purchases().GetByOrderId(d.OrderID)
	if err != nil {
		contract, state, _, records, _, err = i.node.Datastore.Sales().GetByOrderId(d.OrderID)
		if err != nil {
			ErrorResponse(w, http.StatusNotFound, "Order not found")
			return
		}
		isSale = true
	}
	if contract.BuyerOrder.Payment.Method != pb.Order_Payment_MODERATED {
		ErrorResponse(w, http.StatusBadRequest, "Only moderated orders can be disputed")
		return
	}

	if isSale && (state != pb.OrderState_PARTIALLY_FULFILLED && state != pb.OrderState_FULFILLED) {
		ErrorResponse(w, http.StatusBadRequest, "Order must be either PARTIALLY_FULFILLED or FULFILLED to start a dispute")
		return
	}
	if !isSale && !(state == pb.OrderState_AWAITING_FULFILLMENT || state == pb.OrderState_PENDING || state == pb.OrderState_PARTIALLY_FULFILLED || state == pb.OrderState_FULFILLED || state == pb.OrderState_PROCESSING_ERROR) {
		ErrorResponse(w, http.StatusBadRequest, "Order must be either AWAITING_FULFILLMENT, PARTIALLY_FULFILLED, PENDING, PROCESSING_ERROR or FULFILLED to start a dispute")
		return
	}

	if !isSale && state == pb.OrderState_PROCESSING_ERROR && len(records) == 0 {
		ErrorResponse(w, http.StatusBadRequest, "Cannot dispute an unfunded order")
		return
	}

	err = i.node.OpenDispute(d.OrderID, contract, records, d.Claim)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	SanitizedResponse(w, `{}`)
	return
}

func (i *jsonAPIHandler) POSTCloseDispute(w http.ResponseWriter, r *http.Request) {
	type dispute struct {
		OrderID          string  `json:"orderId"`
		Resolution       string  `json:"resolution"`
		BuyerPercentage  float32 `json:"buyerPercentage"`
		VendorPercentage float32 `json:"vendorPercentage"`
	}
	decoder := json.NewDecoder(r.Body)
	var d dispute
	err := decoder.Decode(&d)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	err = i.node.CloseDispute(d.OrderID, d.BuyerPercentage, d.VendorPercentage, d.Resolution)
	if err != nil && err == core.ErrCaseNotFound {
		ErrorResponse(w, http.StatusNotFound, err.Error())
		return
	} else if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	SanitizedResponse(w, `{}`)
	return
}

func (i *jsonAPIHandler) GETCase(w http.ResponseWriter, r *http.Request) {
	_, orderId := path.Split(r.URL.Path)
	buyerContract, vendorContract, buyerErrors, vendorErrors, state, read, date, buyerOpened, claim, resolution, err := i.node.Datastore.Cases().GetCaseMetadata(orderId)
	if err != nil {
		ErrorResponse(w, http.StatusNotFound, err.Error())
		return
	}

	resp := new(pb.CaseRespApi)
	ts, err := ptypes.TimestampProto(date)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	resp.BuyerContract = buyerContract
	resp.VendorContract = vendorContract
	resp.BuyerOpened = buyerOpened
	resp.BuyerContractValidationErrors = buyerErrors
	resp.VendorContractValidationErrors = vendorErrors
	resp.Read = read
	resp.State = state
	resp.Claim = claim
	resp.Resolution = resolution
	resp.Timestamp = ts

	unread, err := i.node.Datastore.Chat().GetUnreadCount(orderId)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	resp.UnreadChatMessages = uint64(unread)

	m := jsonpb.Marshaler{
		EnumsAsInts:  false,
		EmitDefaults: true,
		Indent:       "    ",
		OrigName:     false,
	}
	out, err := m.MarshalToString(resp)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	i.node.Datastore.Cases().MarkAsRead(orderId)
	SanitizedResponseM(w, out, new(pb.CaseRespApi))
}

func (i *jsonAPIHandler) POSTReleaseFunds(w http.ResponseWriter, r *http.Request) {
	type release struct {
		OrderID string `json:"orderId"`
	}
	decoder := json.NewDecoder(r.Body)
	var rel release
	err := decoder.Decode(&rel)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	var contract *pb.RicardianContract
	var state pb.OrderState
	var records []*wallet.TransactionRecord
	contract, state, _, records, _, err = i.node.Datastore.Purchases().GetByOrderId(rel.OrderID)
	if err != nil {
		contract, state, _, records, _, err = i.node.Datastore.Sales().GetByOrderId(rel.OrderID)
		if err != nil {
			ErrorResponse(w, http.StatusNotFound, "Order not found")
			return
		}
	}
	if state == pb.OrderState_DECIDED {
		err = i.node.ReleaseFunds(contract, records)
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
	} else {
		ErrorResponse(w, http.StatusBadRequest, "releasefunds can only be called for decided disputes")
		return
	}
	SanitizedResponse(w, `{}`)
	return
}

func (i *jsonAPIHandler) POSTReleaseEscrow(w http.ResponseWriter, r *http.Request) {
	var (
		rel struct {
			OrderID string `json:"orderId"`
		}
		contract *pb.RicardianContract
		state    pb.OrderState
		records  []*wallet.TransactionRecord
	)

	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&rel)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	contract, state, _, records, _, err = i.node.Datastore.Sales().GetByOrderId(rel.OrderID)
	if err != nil {
		ErrorResponse(w, http.StatusNotFound, "Order not found")
		return
	}

	switch state {
	case pb.OrderState_DISPUTED:
		disputeStart, err := ptypes.Timestamp(contract.GetDispute().Timestamp)
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}

		fourtyFiveDaysInHours := 45 * 24
		disputeDuration := time.Duration(fourtyFiveDaysInHours) * time.Hour

		// Time hack until we can stub this more nicely in test env
		if i.node.TestNetworkEnabled() || i.node.RegressionNetworkEnabled() {
			disputeDuration = time.Duration(10) * time.Second
		}

		disputeTimeout := disputeStart.Add(disputeDuration)
		if time.Now().Before(disputeTimeout) {
			expiresIn := disputeTimeout.Sub(time.Now())
			ErrorResponse(w, http.StatusUnauthorized, fmt.Sprintf("releaseescrow can only be called when in dispute for %s or longer, expires in %s", disputeDuration.String(), expiresIn.String()))
			return
		}
		err = i.node.ReleaseFundsAfterTimeout(contract, records)
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
	case pb.OrderState_FULFILLED:
		err = i.node.ReleaseFundsAfterTimeout(contract, records)
		if err != nil {
			if err == core.EscrowTimeLockedError {
				ErrorResponse(w, http.StatusUnauthorized, err.Error())
				return
			} else {
				ErrorResponse(w, http.StatusInternalServerError, err.Error())
				return
			}
		}
	default:
		ErrorResponse(w, http.StatusBadRequest, "releaseescrow can only be called when in dispute for 45 days or fulfilled for longer than escrow timeout")
		return
	}

	SanitizedResponse(w, `{}`)
	return
}

func (i *jsonAPIHandler) POSTChat(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	var chat repo.ChatMessage
	err := decoder.Decode(&chat)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(chat.Subject) > 500 {
		ErrorResponse(w, http.StatusBadRequest, "Subject line is too long")
		return
	}
	if len(chat.Message) > 20000 {
		ErrorResponse(w, http.StatusBadRequest, "Message is too long")
		return
	}

	t := time.Now()
	ts, err := ptypes.TimestampProto(t)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	var flag pb.Chat_Flag
	if chat.Message == "" {
		flag = pb.Chat_TYPING
	} else {
		flag = pb.Chat_MESSAGE
	}
	h := sha256.Sum256([]byte(chat.Message + chat.Subject + ptypes.TimestampString(ts)))
	encoded, err := mh.Encode(h[:], mh.SHA2_256)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	msgId, err := mh.Cast(encoded)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	chatPb := &pb.Chat{
		MessageId: msgId.B58String(),
		Subject:   chat.Subject,
		Message:   chat.Message,
		Timestamp: ts,
		Flag:      flag,
	}
	err = i.node.SendChat(chat.PeerId, chatPb)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	// Put to database
	if chatPb.Flag == pb.Chat_MESSAGE {
		err = i.node.Datastore.Chat().Put(msgId.B58String(), chat.PeerId, chat.Subject, chat.Message, t, false, true)
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	SanitizedResponse(w, fmt.Sprintf(`{"messageId": "%s"}`, msgId.B58String()))
	return
}

func (i *jsonAPIHandler) POSTGroupChat(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	var chat repo.GroupChatMessage
	err := decoder.Decode(&chat)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(chat.Subject) > 500 {
		ErrorResponse(w, http.StatusBadRequest, "Subject line is too long")
		return
	}
	if len(chat.Subject) <= 0 {
		ErrorResponse(w, http.StatusBadRequest, "Group chats must include a unquie subject to be used as the groupd chat ID")
		return
	}
	if len(chat.Message) > 20000 {
		ErrorResponse(w, http.StatusBadRequest, "Message is too long")
		return
	}

	t := time.Now()
	ts, err := ptypes.TimestampProto(t)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	var flag pb.Chat_Flag
	if chat.Message == "" {
		flag = pb.Chat_TYPING
	} else {
		flag = pb.Chat_MESSAGE
	}
	h := sha256.Sum256([]byte(chat.Message + chat.Subject + ptypes.TimestampString(ts)))
	encoded, err := mh.Encode(h[:], mh.SHA2_256)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	msgId, err := mh.Cast(encoded)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	chatPb := &pb.Chat{
		MessageId: msgId.B58String(),
		Subject:   chat.Subject,
		Message:   chat.Message,
		Timestamp: ts,
		Flag:      flag,
	}
	for _, pid := range chat.PeerIds {
		err = i.node.SendChat(pid, chatPb)
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	// Put to database
	if chatPb.Flag == pb.Chat_MESSAGE {
		err = i.node.Datastore.Chat().Put(msgId.B58String(), "", chat.Subject, chat.Message, t, false, true)
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	SanitizedResponse(w, fmt.Sprintf(`{"messageId": "%s"}`, msgId.B58String()))
	return
}

func (i *jsonAPIHandler) GETChatMessages(w http.ResponseWriter, r *http.Request) {
	_, peerId := path.Split(r.URL.Path)
	if strings.ToLower(peerId) == "chatmessages" {
		peerId = ""
	}
	limit := r.URL.Query().Get("limit")
	if limit == "" {
		limit = "-1"
	}
	l, err := strconv.Atoi(limit)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	offsetId := r.URL.Query().Get("offsetId")
	messages := i.node.Datastore.Chat().GetMessages(peerId, r.URL.Query().Get("subject"), offsetId, int(l))

	ret, err := json.MarshalIndent(messages, "", "    ")
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	if string(ret) == "null" {
		ret = []byte("[]")
	}
	SanitizedResponse(w, string(ret))
	return
}

func (i *jsonAPIHandler) GETChatConversations(w http.ResponseWriter, r *http.Request) {
	conversations := i.node.Datastore.Chat().GetConversations()
	ret, err := json.MarshalIndent(conversations, "", "    ")
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	if string(ret) == "null" {
		ret = []byte("[]")
	}
	SanitizedResponse(w, string(ret))
	return
}

func (i *jsonAPIHandler) POSTMarkChatAsRead(w http.ResponseWriter, r *http.Request) {
	_, peerId := path.Split(r.URL.Path)
	if strings.ToLower(peerId) == "markchatasread" {
		peerId = ""
	}
	subject := r.URL.Query().Get("subject")
	lastId, updated, err := i.node.Datastore.Chat().MarkAsRead(peerId, subject, false, "")
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	if updated && peerId != "" {
		chatPb := &pb.Chat{
			MessageId: lastId,
			Subject:   subject,
			Flag:      pb.Chat_READ,
		}
		err = i.node.SendChat(peerId, chatPb)
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	if subject != "" {
		go func() {
			i.node.Datastore.Purchases().MarkAsRead(subject)
			i.node.Datastore.Sales().MarkAsRead(subject)
			i.node.Datastore.Cases().MarkAsRead(subject)
		}()
	}
	SanitizedResponse(w, `{}`)
}

func (i *jsonAPIHandler) DELETEChatMessage(w http.ResponseWriter, r *http.Request) {
	_, messageId := path.Split(r.URL.Path)
	err := i.node.Datastore.Chat().DeleteMessage(messageId)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	SanitizedResponse(w, `{}`)
}

func (i *jsonAPIHandler) DELETEChatConversation(w http.ResponseWriter, r *http.Request) {
	_, peerId := path.Split(r.URL.Path)
	err := i.node.Datastore.Chat().DeleteConversation(peerId)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	SanitizedResponse(w, `{}`)
}

func (i *jsonAPIHandler) GETNotifications(w http.ResponseWriter, r *http.Request) {
	limit := r.URL.Query().Get("limit")
	if limit == "" {
		limit = "-1"
	}
	l, err := strconv.Atoi(limit)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	offsetId := r.URL.Query().Get("offsetId")
	filter := r.URL.Query().Get("filter")

	types := strings.Split(filter, ",")
	var filters []string
	for _, t := range types {
		if t != "" {
			filters = append(filters, t)
		}
	}

	type notifData struct {
		Unread        int                          `json:"unread"`
		Total         int                          `json:"total"`
		Notifications []notifications.Notification `json:"notifications"`
	}
	notifs, total, err := i.node.Datastore.Notifications().GetAll(offsetId, int(l), filters)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	unread, err := i.node.Datastore.Notifications().GetUnreadCount()
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	ret, err := json.MarshalIndent(notifData{unread, total, notifs}, "", "    ")
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	retString := string(ret)
	if strings.Contains(retString, "null") {
		retString = strings.Replace(retString, "null", "[]", -1)
	}
	SanitizedResponse(w, retString)
	return
}

func (i *jsonAPIHandler) POSTMarkNotificationAsRead(w http.ResponseWriter, r *http.Request) {
	_, notifId := path.Split(r.URL.Path)
	err := i.node.Datastore.Notifications().MarkAsRead(notifId)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	SanitizedResponse(w, `{}`)
}

func (i *jsonAPIHandler) POSTMarkNotificationsAsRead(w http.ResponseWriter, r *http.Request) {
	err := i.node.Datastore.Notifications().MarkAllAsRead()
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	SanitizedResponse(w, `{}`)
}

func (i *jsonAPIHandler) DELETENotification(w http.ResponseWriter, r *http.Request) {
	_, notifId := path.Split(r.URL.Path)
	err := i.node.Datastore.Notifications().Delete(notifId)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	SanitizedResponse(w, `{}`)
}

func (i *jsonAPIHandler) GETImage(w http.ResponseWriter, r *http.Request) {
	_, imageHash := path.Split(r.URL.Path)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*2)
	defer cancel()
	dr, err := coreunix.Cat(ctx, i.node.IpfsNode, "/ipfs/"+imageHash)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer dr.Close()
	w.Header().Set("Cache-Control", "public, max-age=29030400, immutable")
	w.Header().Del("Content-Type")
	http.ServeContent(w, r, imageHash, time.Now(), dr)
}

func (i *jsonAPIHandler) GETAvatar(w http.ResponseWriter, r *http.Request) {
	urlPath, size := path.Split(r.URL.Path)
	_, peerId := path.Split(urlPath[:len(urlPath)-1])

	cacheBool := r.URL.Query().Get("usecache")
	useCache, _ := strconv.ParseBool(cacheBool)

	dr, err := i.node.FetchAvatar(peerId, size, useCache)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer dr.Close()
	w.Header().Set("Cache-Control", "public, max-age=600, immutable")
	w.Header().Del("Content-Type")
	http.ServeContent(w, r, path.Join("ipns", peerId, "images", size, "avatar"), time.Now(), dr)
}

func (i *jsonAPIHandler) GETHeader(w http.ResponseWriter, r *http.Request) {
	urlPath, size := path.Split(r.URL.Path)
	_, peerId := path.Split(urlPath[:len(urlPath)-1])

	cacheBool := r.URL.Query().Get("usecache")
	useCache, _ := strconv.ParseBool(cacheBool)

	dr, err := i.node.FetchHeader(peerId, size, useCache)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer dr.Close()
	w.Header().Set("Cache-Control", "public, max-age=600, immutable")
	w.Header().Del("Content-Type")
	http.ServeContent(w, r, path.Join("ipns", peerId, "images", size, "header"), time.Now(), dr)
}

func (i *jsonAPIHandler) POSTFetchProfiles(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("async")
	async, _ := strconv.ParseBool(query)
	cacheBool := r.URL.Query().Get("usecache")
	useCache, _ := strconv.ParseBool(cacheBool)

	var pids []string
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&pids)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	if !async {
		var wg sync.WaitGroup
		var ret []string
		for _, p := range pids {
			wg.Add(1)
			go func(pid string) {
				pro, err := i.node.FetchProfile(pid, useCache)
				if err != nil {
					wg.Done()
					return
				}
				obj := pb.PeerAndProfile{pid, &pro}
				m := jsonpb.Marshaler{
					EnumsAsInts:  false,
					EmitDefaults: true,
					Indent:       "    ",
					OrigName:     false,
				}
				respJson, err := m.MarshalToString(&obj)
				if err != nil {
					return
				}
				ret = append(ret, respJson)
				wg.Done()
			}(p)
		}
		wg.Wait()
		resp := "[\n"
		max := len(ret)
		for i, r := range ret {
			lines := strings.Split(r, "\n")
			maxx := len(lines)
			for x, s := range lines {
				resp += "    "
				resp += s
				if x != maxx-1 {
					resp += "\n"
				}
			}
			if i != max-1 {
				resp += ",\n"
			}
		}
		resp += "\n]"
		SanitizedResponse(w, resp)
	} else {
		idBytes := make([]byte, 16)
		rand.Read(idBytes)
		id := base58.Encode(idBytes)

		type resp struct {
			Id string `json:"id"`
		}
		response := resp{id}
		respJson, _ := json.MarshalIndent(response, "", "    ")
		w.WriteHeader(http.StatusAccepted)
		SanitizedResponse(w, string(respJson))
		go func() {
			type profileError struct {
				ID     string `json:"id"`
				PeerId string `json:"peerId"`
				Error  string `json:"error"`
			}
			for _, p := range pids {
				go func(pid string) {
					respondWithError := func(errorMsg string) {
						e := profileError{id, pid, errorMsg}
						ret, err := json.MarshalIndent(e, "", "    ")
						if err != nil {
							return
						}
						i.node.Broadcast <- ret
						return
					}

					pro, err := i.node.FetchProfile(pid, useCache)
					if err != nil {
						respondWithError("Not found")
						return
					}
					obj := pb.PeerAndProfileWithID{id, pid, &pro}
					m := jsonpb.Marshaler{
						EnumsAsInts:  false,
						EmitDefaults: true,
						Indent:       "    ",
						OrigName:     false,
					}
					respJson, err := m.MarshalToString(&obj)
					if err != nil {
						respondWithError("Error Marshalling to JSON")
						return
					}
					b, err := SanitizeProtobuf(respJson, new(pb.PeerAndProfileWithID))
					if err != nil {
						respondWithError("Error Marshalling to JSON")
						return
					}
					i.node.Broadcast <- b
				}(p)
			}
		}()
	}
}

func (i *jsonAPIHandler) GETTransactions(w http.ResponseWriter, r *http.Request) {
	l := r.URL.Query().Get("limit")
	if l == "" {
		l = "-1"
	}
	limit, err := strconv.Atoi(l)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	offsetID := r.URL.Query().Get("offsetId")
	type Tx struct {
		Txid          string    `json:"txid"`
		Value         int64     `json:"value"`
		Address       string    `json:"address"`
		Status        string    `json:"status"`
		Memo          string    `json:"memo"`
		Timestamp     time.Time `json:"timestamp"`
		Confirmations int32     `json:"confirmations"`
		Height        int32     `json:"height"`
		OrderId       string    `json:"orderId"`
		Thumbnail     string    `json:"thumbnail"`
		CanBumpFee    bool      `json:"canBumpFee"`
	}
	transactions, err := i.node.Wallet.Transactions()
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	metadata, err := i.node.Datastore.TxMetadata().GetAll()
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	height, _ := i.node.Wallet.ChainTip()
	var txs []Tx
	passedOffset := false
	for i := len(transactions) - 1; i >= 0; i-- {
		t := transactions[i]
		var confirmations int32
		var status string
		confs := int32(height) - t.Height + 1
		if t.Height <= 0 {
			confs = t.Height
		}
		switch {
		case confs < 0:
			status = "DEAD"
		case confs == 0 && time.Since(t.Timestamp) <= time.Hour*6:
			status = "UNCONFIRMED"
		case confs == 0 && time.Since(t.Timestamp) > time.Hour*6:
			status = "STUCK"
		case confs > 0 && confs < 6:
			status = "PENDING"
			confirmations = confs
		case confs > 5:
			status = "CONFIRMED"
			confirmations = confs
		}
		tx := Tx{
			Txid:          t.Txid,
			Value:         t.Value,
			Timestamp:     t.Timestamp,
			Confirmations: confirmations,
			Height:        t.Height,
			Status:        status,
			CanBumpFee:    true,
		}
		m, ok := metadata[t.Txid]
		if ok {
			tx.Address = m.Address
			tx.Memo = m.Memo
			tx.OrderId = m.OrderId
			tx.Thumbnail = m.Thumbnail
			tx.CanBumpFee = m.CanBumpFee
		}
		if status == "DEAD" {
			tx.CanBumpFee = false
		}
		if offsetID == "" || passedOffset {
			txs = append(txs, tx)
		}
		if t.Txid == offsetID {
			passedOffset = true
		}
		if len(txs) >= limit && limit != -1 {
			break
		}
	}
	type txWithCount struct {
		Transactions []Tx `json:"transactions"`
		Count        int  `json:"count"`
	}
	txns := txWithCount{txs, len(transactions)}
	ret, err := json.MarshalIndent(txns, "", "    ")
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	SanitizedResponse(w, string(ret))
}

func (i *jsonAPIHandler) GETPurchases(w http.ResponseWriter, r *http.Request) {
	orderStates, searchTerm, sortByAscending, sortByRead, limit, err := parseSearchTerms(r.URL.Query())
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	purchases, queryCount, err := i.node.Datastore.Purchases().GetAll(orderStates, searchTerm, sortByAscending, sortByRead, limit, []string{})
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	for n, p := range purchases {
		unread, err := i.node.Datastore.Chat().GetUnreadCount(p.OrderId)
		if err != nil {
			continue
		}
		purchases[n].UnreadChatMessages = unread
	}
	type purchasesResponse struct {
		QueryCount int             `json:"queryCount"`
		Purchases  []repo.Purchase `json:"purchases"`
	}
	pr := purchasesResponse{queryCount, purchases}
	ret, err := json.MarshalIndent(pr, "", "    ")
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	if string(ret) == "null" {
		ret = []byte("[]")
	}
	SanitizedResponse(w, string(ret))
	return
}

func (i *jsonAPIHandler) GETSales(w http.ResponseWriter, r *http.Request) {
	orderStates, searchTerm, sortByAscending, sortByRead, limit, err := parseSearchTerms(r.URL.Query())
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	sales, queryCount, err := i.node.Datastore.Sales().GetAll(orderStates, searchTerm, sortByAscending, sortByRead, limit, []string{})
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	for n, s := range sales {
		unread, err := i.node.Datastore.Chat().GetUnreadCount(s.OrderId)
		if err != nil {
			continue
		}
		sales[n].UnreadChatMessages = unread
	}
	type salesResponse struct {
		QueryCount int         `json:"queryCount"`
		Sales      []repo.Sale `json:"sales"`
	}
	sr := salesResponse{queryCount, sales}

	ret, err := json.MarshalIndent(sr, "", "    ")
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	if string(ret) == "null" {
		ret = []byte("[]")
	}
	SanitizedResponse(w, string(ret))
	return
}

func (i *jsonAPIHandler) GETCases(w http.ResponseWriter, r *http.Request) {
	orderStates, searchTerm, sortByAscending, sortByRead, limit, err := parseSearchTerms(r.URL.Query())
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	cases, queryCount, err := i.node.Datastore.Cases().GetAll(orderStates, searchTerm, sortByAscending, sortByRead, limit, []string{})
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	for n, c := range cases {
		unread, err := i.node.Datastore.Chat().GetUnreadCount(c.CaseId)
		if err != nil {
			continue
		}
		cases[n].UnreadChatMessages = unread
	}
	type casesResponse struct {
		QueryCount int         `json:"queryCount"`
		Cases      []repo.Case `json:"cases"`
	}
	cr := casesResponse{queryCount, cases}
	ret, err := json.MarshalIndent(cr, "", "    ")
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	if string(ret) == "null" {
		ret = []byte("[]")
	}
	SanitizedResponse(w, string(ret))
	return
}

func (i *jsonAPIHandler) POSTPurchases(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	var query TransactionQuery
	err := decoder.Decode(&query)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	purchases, queryCount, err := i.node.Datastore.Purchases().GetAll(convertOrderStates(query.OrderStates), query.SearchTerm, query.SortByAscending, query.SortByRead, query.Limit, query.Exclude)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	for n, p := range purchases {
		unread, err := i.node.Datastore.Chat().GetUnreadCount(p.OrderId)
		if err != nil {
			continue
		}
		purchases[n].UnreadChatMessages = unread
	}
	type purchasesResponse struct {
		QueryCount int             `json:"queryCount"`
		Purchases  []repo.Purchase `json:"purchases"`
	}
	pr := purchasesResponse{queryCount, purchases}
	ret, err := json.MarshalIndent(pr, "", "    ")
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	if string(ret) == "null" {
		ret = []byte("[]")
	}
	SanitizedResponse(w, string(ret))
	return
}

func (i *jsonAPIHandler) POSTSales(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	var query TransactionQuery
	err := decoder.Decode(&query)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	sales, queryCount, err := i.node.Datastore.Sales().GetAll(convertOrderStates(query.OrderStates), query.SearchTerm, query.SortByAscending, query.SortByRead, query.Limit, query.Exclude)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	for n, s := range sales {
		unread, err := i.node.Datastore.Chat().GetUnreadCount(s.OrderId)
		if err != nil {
			continue
		}
		sales[n].UnreadChatMessages = unread
	}
	type salesResponse struct {
		QueryCount int         `json:"queryCount"`
		Sales      []repo.Sale `json:"sales"`
	}
	sr := salesResponse{queryCount, sales}

	ret, err := json.MarshalIndent(sr, "", "    ")
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	if string(ret) == "null" {
		ret = []byte("[]")
	}
	SanitizedResponse(w, string(ret))
	return
}

func (i *jsonAPIHandler) POSTCases(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	var query TransactionQuery
	err := decoder.Decode(&query)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	cases, queryCount, err := i.node.Datastore.Cases().GetAll(convertOrderStates(query.OrderStates), query.SearchTerm, query.SortByAscending, query.SortByRead, query.Limit, query.Exclude)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	for n, c := range cases {
		unread, err := i.node.Datastore.Chat().GetUnreadCount(c.CaseId)
		if err != nil {
			continue
		}
		cases[n].UnreadChatMessages = unread
	}
	type casesResponse struct {
		QueryCount int         `json:"queryCount"`
		Cases      []repo.Case `json:"cases"`
	}
	cr := casesResponse{queryCount, cases}
	ret, err := json.MarshalIndent(cr, "", "    ")
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	if string(ret) == "null" {
		ret = []byte("[]")
	}
	SanitizedResponse(w, string(ret))
	return
}

func (i *jsonAPIHandler) POSTBlockNode(w http.ResponseWriter, r *http.Request) {
	_, peerId := path.Split(r.URL.Path)
	settings, err := i.node.Datastore.Settings().Get()
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	var nodes []string
	if settings.BlockedNodes != nil {
		for _, pid := range *settings.BlockedNodes {
			if pid == peerId {
				fmt.Fprint(w, `{}`)
				return
			}
			nodes = append(nodes, pid)
		}
	}
	go ipfs.RemoveAll(i.node.Context, peerId)
	nodes = append(nodes, peerId)
	settings.BlockedNodes = &nodes
	if err := i.node.Datastore.Settings().Put(settings); err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	pid, err := peer.IDB58Decode(peerId)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	i.node.BanManager.AddBlockedId(pid)
	SanitizedResponse(w, `{}`)
}

func (i *jsonAPIHandler) DELETEBlockNode(w http.ResponseWriter, r *http.Request) {
	_, peerId := path.Split(r.URL.Path)
	settings, err := i.node.Datastore.Settings().Get()
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	if settings.BlockedNodes != nil {
		var nodes []string
		for _, pid := range *settings.BlockedNodes {
			if pid != peerId {
				nodes = append(nodes, pid)
			}
		}
		settings.BlockedNodes = &nodes
	}
	if err := i.node.Datastore.Settings().Put(settings); err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	pid, err := peer.IDB58Decode(peerId)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	i.node.BanManager.RemoveBlockedId(pid)
	SanitizedResponse(w, `{}`)
}

func (i *jsonAPIHandler) POSTBumpFee(w http.ResponseWriter, r *http.Request) {
	_, txid := path.Split(r.URL.Path)
	txHash, err := chainhash.NewHashFromStr(txid)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	newTxid, err := i.node.Wallet.BumpFee(*txHash)
	if err != nil {
		if err == spvwallet.BumpFeeAlreadyConfirmedError {
			ErrorResponse(w, http.StatusBadRequest, err.Error())
		} else if err == spvwallet.BumpFeeTransactionDeadError {
			ErrorResponse(w, http.StatusMethodNotAllowed, err.Error())
		} else if err == spvwallet.BumpFeeNotFoundError {
			ErrorResponse(w, http.StatusNotFound, err.Error())
		} else {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	m, err := i.node.Datastore.TxMetadata().Get(txid)
	if err != nil {
		m = repo.Metadata{}
	}
	m.Txid = txid
	m.CanBumpFee = false
	if err := i.node.Datastore.TxMetadata().Put(m); err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := i.node.Datastore.TxMetadata().Put(repo.Metadata{
		Txid:       newTxid.String(),
		Address:    "",
		Memo:       fmt.Sprintf("Fee bump of %s", txid),
		OrderId:    "",
		Thumbnail:  "",
		CanBumpFee: true,
	}); err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	type response struct {
		Txid               string    `json:"txid"`
		Amount             int64     `json:"amount"`
		ConfirmedBalance   int64     `json:"confirmedBalance"`
		UnconfirmedBalance int64     `json:"unconfirmedBalance"`
		Timestamp          time.Time `json:"timestamp"`
		Memo               string    `json:"memo"`
	}
	confirmed, unconfirmed := i.node.Wallet.Balance()
	txn, err := i.node.Wallet.GetTransaction(*newTxid)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	resp := &response{
		Txid:               newTxid.String(),
		ConfirmedBalance:   confirmed,
		UnconfirmedBalance: unconfirmed,
		Amount:             -(txn.Value),
		Timestamp:          txn.Timestamp,
		Memo:               fmt.Sprintf("Fee bump of %s", txid),
	}
	ser, err := json.MarshalIndent(resp, "", "    ")
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	SanitizedResponse(w, string(ser))
}

func (i *jsonAPIHandler) GETEstimateFee(w http.ResponseWriter, r *http.Request) {
	fl := r.URL.Query().Get("feeLevel")
	amt := r.URL.Query().Get("amount")
	amount, err := strconv.Atoi(amt)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	var feeLevel wallet.FeeLevel
	switch strings.ToUpper(fl) {
	case "PRIORITY":
		feeLevel = wallet.PRIOIRTY
	case "NORMAL":
		feeLevel = wallet.NORMAL
	case "ECONOMIC":
		feeLevel = wallet.ECONOMIC
	default:
		ErrorResponse(w, http.StatusBadRequest, "Unknown feeLevel")
		return
	}

	fee, err := i.node.Wallet.EstimateSpendFee(int64(amount), feeLevel)
	if err != nil {
		switch {
		case err == wallet.ErrorInsuffientFunds:
			ErrorResponse(w, http.StatusBadRequest, `ERROR_INSUFFICIENT_FUNDS`)
			return
		case err == wallet.ErrorDustAmount:
			ErrorResponse(w, http.StatusBadRequest, `ERROR_DUST_AMOUNT`)
			return
		default:
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	fmt.Fprintf(w, `{"estimatedFee": %d}`, (fee))
	return
}

func (i *jsonAPIHandler) GETFees(w http.ResponseWriter, r *http.Request) {
	priority := i.node.Wallet.GetFeePerByte(wallet.PRIOIRTY)
	normal := i.node.Wallet.GetFeePerByte(wallet.NORMAL)
	economic := i.node.Wallet.GetFeePerByte(wallet.ECONOMIC)
	fmt.Fprintf(w, `{"priority": %d, "normal": %d, "economic": %d}`, int(priority), int(normal), int(economic))
	return
}

func (i *jsonAPIHandler) POSTEstimateTotal(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	var data core.PurchaseData
	err := decoder.Decode(&data)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	amount, err := i.node.EstimateOrderTotal(&data)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	fmt.Fprintf(w, "%d", int(amount))
	return
}

func (i *jsonAPIHandler) GETRatings(w http.ResponseWriter, r *http.Request) {
	urlPath, slug := path.Split(r.URL.Path)
	_, peerId := path.Split(urlPath[:len(urlPath)-1])

	if peerId == "ratings" {
		peerId = slug
		slug = ""
	}

	var indexBytes []byte
	if peerId != i.node.IPFSIdentityString() {
		indexBytes, _ = i.node.IPNSResolveThenCat(ipnspath.FromString(path.Join(peerId, "ratings.json")), time.Minute)

	} else {
		indexBytes, _ = ioutil.ReadFile(path.Join(i.node.RepoPath, "root", "ratings.json"))
	}
	if indexBytes == nil {
		rating := new(core.SavedRating)
		rating.Ratings = []string{}
		ret, err := json.MarshalIndent(rating, "", "    ")
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		SanitizedResponse(w, string(ret))
		return
	}

	var ratingList []core.SavedRating
	err := json.Unmarshal(indexBytes, &ratingList)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	if slug != "" {
		rating := new(core.SavedRating)
		for _, r := range ratingList {
			if r.Slug == slug {
				rating = &r
				break
			}
		}
		ret, err := json.MarshalIndent(rating, "", "    ")
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		SanitizedResponse(w, string(ret))
	} else {
		type resp struct {
			Count   int      `json:"count"`
			Average float32  `json:"average"`
			Ratings []string `json:"ratings"`
		}
		ratingRet := new(resp)
		total := float32(0)
		count := 0
		for _, r := range ratingList {
			total += r.Average * float32(r.Count)
			count += r.Count
			ratingRet.Ratings = append(ratingRet.Ratings, r.Ratings...)
		}
		ratingRet.Count = count
		ratingRet.Average = total / float32(count)
		ret, err := json.MarshalIndent(ratingRet, "", "    ")
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		SanitizedResponse(w, string(ret))
	}
}

func (i *jsonAPIHandler) GETRating(w http.ResponseWriter, r *http.Request) {
	_, ratingID := path.Split(r.URL.Path)

	ratingBytes, err := ipfs.Cat(i.node.Context, ratingID, time.Minute)
	if err != nil {
		ErrorResponse(w, http.StatusNotFound, err.Error())
		return
	}

	rating := new(pb.Rating)
	err = jsonpb.UnmarshalString(string(ratingBytes), rating)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	valid, err := core.ValidateRating(rating)
	if !valid || err != nil {
		ErrorResponse(w, http.StatusExpectationFailed, err.Error())
		return
	}
	ret, err := json.MarshalIndent(rating, "", "    ")
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	SanitizedResponse(w, string(ret))
}

func (i *jsonAPIHandler) POSTFetchRatings(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	var rp []string
	err := decoder.Decode(&rp)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	query := r.URL.Query().Get("async")
	async, _ := strconv.ParseBool(query)

	if !async {
		var wg sync.WaitGroup
		var ret []string
		for _, id := range rp {
			wg.Add(1)
			go func(rid string) {
				ratingBytes, err := ipfs.Cat(i.node.Context, rid, time.Minute)
				if err != nil {
					return
				}
				rating := new(pb.Rating)
				err = jsonpb.UnmarshalString(string(ratingBytes), rating)
				if err != nil {
					return
				}
				valid, err := core.ValidateRating(rating)
				if !valid || err != nil {
					return
				}
				m := jsonpb.Marshaler{
					EnumsAsInts:  false,
					EmitDefaults: true,
					Indent:       "    ",
					OrigName:     false,
				}
				respJson, err := m.MarshalToString(rating)
				if err != nil {
					return
				}
				ret = append(ret, respJson)
				wg.Done()
			}(id)
		}
		wg.Wait()
		resp := "[\n"
		max := len(ret)
		for i, r := range ret {
			lines := strings.Split(r, "\n")
			maxx := len(lines)
			for x, s := range lines {
				resp += "    "
				resp += s
				if x != maxx-1 {
					resp += "\n"
				}
			}
			if i != max-1 {
				resp += ",\n"
			}
		}
		resp += "\n]"
		SanitizedResponse(w, resp)
	} else {
		idBytes := make([]byte, 16)
		rand.Read(idBytes)
		id := base58.Encode(idBytes)

		type resp struct {
			Id string `json:"id"`
		}
		response := resp{id}
		respJson, _ := json.MarshalIndent(response, "", "    ")
		w.WriteHeader(http.StatusAccepted)
		SanitizedResponse(w, string(respJson))
		for _, r := range rp {
			go func(rid string) {
				type ratingError struct {
					ID       string `json:"id"`
					RatingID string `json:"ratingId"`
					Error    string `json:"error"`
				}
				respondWithError := func(errorMsg string) {
					e := ratingError{id, rid, "Not found"}
					ret, err := json.MarshalIndent(e, "", "    ")
					if err != nil {
						return
					}
					i.node.Broadcast <- ret
					return
				}
				ratingBytes, err := ipfs.Cat(i.node.Context, rid, time.Minute)
				if err != nil {
					respondWithError("Not Found")
					return
				}

				rating := new(pb.Rating)
				err = jsonpb.UnmarshalString(string(ratingBytes), rating)
				if err != nil {
					respondWithError("Invalid rating")
					return
				}
				valid, err := core.ValidateRating(rating)
				if !valid || err != nil {
					respondWithError(err.Error())
					return
				}
				resp := new(pb.RatingWithID)
				resp.Id = id
				resp.RatingId = rid
				resp.Rating = rating
				m := jsonpb.Marshaler{
					EnumsAsInts:  false,
					EmitDefaults: true,
					Indent:       "    ",
					OrigName:     false,
				}
				out, err := m.MarshalToString(resp)
				if err != nil {
					respondWithError("Error marshalling rating")
					return
				}
				b, err := SanitizeProtobuf(out, new(pb.RatingWithID))
				if err != nil {
					respondWithError("Error marshalling rating")
					return
				}
				i.node.Broadcast <- b
			}(r)
		}
	}
}

func (i *jsonAPIHandler) POSTImportListings(w http.ResponseWriter, r *http.Request) {
	file, _, err := r.FormFile("file")
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	defer file.Close()

	err = i.node.ImportListings(file)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	// Republish to IPNS
	if err := i.node.SeedNode(); err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	SanitizedResponse(w, "{}")
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
	peers, perr := ipfs.ConnectedPeers(i.node.Context)
	if perr != nil || len(peers) == 0 {
		re.Peers = false
	}
	if pingErr != nil || ferr != nil || perr != nil {
		ret, _ := json.MarshalIndent(re, "", "    ")
		ErrorResponse(w, http.StatusNotFound, string(ret))
		return
	}
	SanitizedResponse(w, "{}")
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

func (i *jsonAPIHandler) GETWalletStatus(w http.ResponseWriter, r *http.Request) {
	height, hash := i.node.Wallet.ChainTip()
	type status struct {
		Height   uint32 `json:"height"`
		BestHash string `json:"bestHash"`
	}
	hh := status{height, hash.String()}
	ret, err := json.MarshalIndent(&hh, "", "    ")
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	SanitizedResponse(w, string(ret))
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
	_, peerId := path.Split(r.URL.Path)

	val, err := i.node.IpfsNode.Repo.Datastore().Get(ds.NewKey(core.CachePrefix + peerId))
	if err != nil { // No record in datastore
		ErrorResponse(w, http.StatusNotFound, err.Error())
		return
	}
	pid, err := peer.IDB58Decode(peerId)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	var keyBytes []byte
	pubkey := i.node.IpfsNode.Peerstore.PubKey(pid)
	if pubkey == nil || !pid.MatchesPublicKey(pubkey) {
		keyval, err := i.node.IpfsNode.Repo.Datastore().Get(ds.NewKey(core.KeyCachePrefix + peerId))
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

	go ipfs.Resolve(i.node.Context, peerId, time.Minute)
	fmt.Fprint(w, string(retBytes))
}

func (i *jsonAPIHandler) POSTTestEmailNotifications(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	var settings repo.SMTPSettings
	err := decoder.Decode(&settings)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	notifier := smtpNotifier{&settings}
	err = notifier.notify(notifications.TestNotification{})
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
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
	ctx, _ := context.WithTimeout(context.Background(), time.Second*30)
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

// POSTS

// Post a post
func (i *jsonAPIHandler) POSTPost(w http.ResponseWriter, r *http.Request) {
	ld := new(pb.Post)
	err := jsonpb.Unmarshal(r.Body, ld)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	// If the post already exists in path, tell them to use PUT
	postPath := path.Join(i.node.RepoPath, "root", "posts", ld.Slug+".json")
	if ld.Slug != "" {
		_, ferr := os.Stat(postPath)
		if !os.IsNotExist(ferr) {
			ErrorResponse(w, http.StatusConflict, "Post already exists. Use PUT.")
			return
		}
	} else {
		// The post isn't in the path and is new, therefore add required data (slug, timestamp)
		// Generate a slug from the title
		ld.Slug, err = i.node.GeneratePostSlug(ld.Title)
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	// Add the timestamp
	ld.Timestamp, err = ptypes.TimestampProto(time.Now())
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	// Sign the post
	signedPost, err := i.node.SignPost(ld)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	// Add to path
	postPath = path.Join(i.node.RepoPath, "root", "posts", signedPost.Post.Slug+".json")
	f, err := os.Create(postPath)
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
	out, err := m.MarshalToString(signedPost)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	if _, err := f.WriteString(out); err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	err = i.node.UpdatePostIndex(signedPost)
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
	SanitizedResponse(w, fmt.Sprintf(`{"slug": "%s"}`, signedPost.Post.Slug))
	return
}

// PUT a post
func (i *jsonAPIHandler) PUTPost(w http.ResponseWriter, r *http.Request) {
	ld := new(pb.Post)
	err := jsonpb.Unmarshal(r.Body, ld)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	// Check if the post exists
	postPath := path.Join(i.node.RepoPath, "root", "posts", ld.Slug+".json")
	_, ferr := os.Stat(postPath)
	if os.IsNotExist(ferr) {
		ErrorResponse(w, http.StatusNotFound, "Post not found.")
		return
	}
	// Add the timestamp
	ld.Timestamp, err = ptypes.TimestampProto(time.Now())
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	// Sign the post
	signedPost, err := i.node.SignPost(ld)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	f, err := os.Create(postPath)
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
	out, err := m.MarshalToString(signedPost)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	if _, err := f.WriteString(out); err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	err = i.node.UpdatePostIndex(signedPost)
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
	if err := i.node.SeedNode(); err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	SanitizedResponse(w, `{}`)
	return
}

// DELETE a post
func (i *jsonAPIHandler) DELETEPost(w http.ResponseWriter, r *http.Request) {
	_, slug := path.Split(r.URL.Path)
	postPath := path.Join(i.node.RepoPath, "root", "posts", slug+".json")
	_, ferr := os.Stat(postPath)
	if os.IsNotExist(ferr) {
		ErrorResponse(w, http.StatusNotFound, "Post not found.")
		return
	}
	err := i.node.DeletePost(slug)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	err = i.node.UpdateFollow()
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, "File Write Error: "+err.Error())
		return
	}
	if err := i.node.SeedNode(); err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	SanitizedResponse(w, `{}`)
	return
}

// GET a list of posts (self or peer)
func (i *jsonAPIHandler) GETPosts(w http.ResponseWriter, r *http.Request) {
	_, peerId := path.Split(r.URL.Path)
	if peerId == "" || strings.ToLower(peerId) == "posts" || peerId == i.node.IPFSIdentityString() {
		postsBytes, err := i.node.GetPosts()
		if err != nil {
			ErrorResponse(w, http.StatusNotFound, err.Error())
			return
		}
		SanitizedResponse(w, string(postsBytes))
	} else {
		pid, err := i.node.NameSystem.Resolve(context.Background(), peerId)
		if err != nil {
			ErrorResponse(w, http.StatusNotFound, err.Error())
			return
		}
		peerId = pid.Pretty()
		postsBytes, err := i.node.IPNSResolveThenCat(ipnspath.FromString(path.Join(peerId, "posts.json")), time.Minute)
		if err != nil {
			ErrorResponse(w, http.StatusNotFound, err.Error())
			return
		}
		SanitizedResponse(w, string(postsBytes))
		w.Header().Set("Cache-Control", "public, max-age=600, immutable")
	}
}

// GET a post (self or peer)
func (i *jsonAPIHandler) GETPost(w http.ResponseWriter, r *http.Request) {
	urlPath, postId := path.Split(r.URL.Path)
	_, peerId := path.Split(urlPath[:len(urlPath)-1])
	m := jsonpb.Marshaler{
		EnumsAsInts:  false,
		EmitDefaults: false,
		Indent:       "    ",
		OrigName:     false,
	}
	if peerId == "" || strings.ToLower(peerId) == "post" || peerId == i.node.IPFSIdentityString() {
		sl := new(pb.SignedPost)
		_, err := cid.Decode(postId)
		if err == nil {
			sl, err = i.node.GetPostFromHash(postId)
			if err != nil {
				ErrorResponse(w, http.StatusNotFound, "Post not found.")
				return
			}
			sl.Hash = postId
		} else {
			sl, err = i.node.GetPostFromSlug(postId)
			if err != nil {
				ErrorResponse(w, http.StatusNotFound, "Post not found.")
				return
			}
			hash, err := ipfs.GetHashOfFile(i.node.Context, path.Join(i.node.RepoPath, "root", "posts", postId+".json"))
			if err != nil {
				ErrorResponse(w, http.StatusInternalServerError, err.Error())
				return
			}
			sl.Hash = hash
		}

		out, err := m.MarshalToString(sl)
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		SanitizedResponseM(w, string(out), new(pb.SignedPost))
		return
	} else {
		var postBytes []byte
		var hash string
		_, err := cid.Decode(postId)
		if err == nil {
			postBytes, err = ipfs.Cat(i.node.Context, postId, time.Minute)
			if err != nil {
				ErrorResponse(w, http.StatusNotFound, err.Error())
				return
			}
			hash = postId
			w.Header().Set("Cache-Control", "public, max-age=29030400, immutable")
		} else {
			pid, err := i.node.NameSystem.Resolve(context.Background(), peerId)
			if err != nil {
				ErrorResponse(w, http.StatusNotFound, err.Error())
				return
			}
			peerId = pid.Pretty()
			postBytes, err = i.node.IPNSResolveThenCat(ipnspath.FromString(path.Join(peerId, "posts", postId+".json")), time.Minute)
			if err != nil {
				ErrorResponse(w, http.StatusNotFound, err.Error())
				return
			}
			hash, err = ipfs.GetHash(i.node.Context, bytes.NewReader(postBytes))
			if err != nil {
				ErrorResponse(w, http.StatusInternalServerError, err.Error())
				return
			}
			w.Header().Set("Cache-Control", "public, max-age=600, immutable")
		}
		sl := new(pb.SignedPost)
		err = jsonpb.UnmarshalString(string(postBytes), sl)
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		sl.Hash = hash
		out, err := m.MarshalToString(sl)
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		SanitizedResponseM(w, out, new(pb.SignedPost))
	}
}
