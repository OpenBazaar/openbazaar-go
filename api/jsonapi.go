package api

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/big"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	ipnspath "gx/ipfs/QmQAgv6Gaoe2tQpcabqwKXKChp2MZ7i3UXv9DqTTaxCaTR/go-path"
	files "gx/ipfs/QmQmhotPUzVrMEWNK3x1R5jQ5ZHWyL7tVUrmRPjrBrvyCb/go-ipfs-files"
	cid "gx/ipfs/QmTbxNB1NwDesLmKTscr4udL2tVP7MaxvXnD1D9yX7g3PN/go-cid"
	ipns "gx/ipfs/QmUwMnKKjH3JwGKNVZ3TcP37W93xzqNA4ECFFiMo6sXkkc/go-ipns"
	iface "gx/ipfs/QmXLwxifxwfc2bAwq6rdjbYqAsGzWsDE9RM5TWMGtykyj6/interface-go-ipfs-core"
	peer "gx/ipfs/QmYVXrKrKHDC9FobgmcmshCDyWwdrfwfanNQN4oxJ9Fk3h/go-libp2p-peer"
	routing "gx/ipfs/QmYxUdYY9S6yg5tSPVin5GFTvtfsLauVcr7reHDD3dM8xf/go-libp2p-routing"
	ps "gx/ipfs/QmaCTz9RkrU13bm9kMB54f7atgqM4qkjDZpRwRoJiWXEqs/go-libp2p-peerstore"
	ggproto "gx/ipfs/QmddjPSGZb3ieihSseFeCfVRpZzcqczPNsD2DvarSwnjJB/gogo-protobuf/proto"
	mh "gx/ipfs/QmerPMzPk1mJVowm8KgmoknWa4yCYvvugMPsgWmDNUvDLW/go-multihash"

	"github.com/OpenBazaar/jsonpb"
	"github.com/OpenBazaar/openbazaar-go/core"
	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/openbazaar-go/schema"
	"github.com/OpenBazaar/spvwallet"
	"github.com/OpenBazaar/wallet-interface"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcutil/base58"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	ipfscore "github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/core/coreapi"
	"github.com/ipfs/go-ipfs/repo/fsrepo"
)

type JSONAPIConfig struct {
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
	config JSONAPIConfig
	node   *core.OpenBazaarNode
}

type APIError struct {
	Success bool   `json:"success"`
	Reason  string `json:"reason"`
}

var lastManualScan time.Time

const OfflineMessageScanInterval = 1 * time.Minute

func newJSONAPIHandler(node *core.OpenBazaarNode, authCookie http.Cookie, config schema.APIConfig) *jsonAPIHandler {
	allowedIPs := make(map[string]bool)
	for _, ip := range config.AllowedIPs {
		allowedIPs[ip] = true
	}
	i := &jsonAPIHandler{
		config: JSONAPIConfig{
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
	return i
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
		w.Header().Set("Access-Control-Allow-Methods", "PUT,POST,PATCH,DELETE,GET,OPTIONS")
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
			if !ok || username != i.config.Username || !strings.EqualFold(password, i.config.Password) {
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
	case "HEAD":
		get(i, u.String(), w, r)
	}
}

func ErrorResponse(w http.ResponseWriter, errorCode int, reason string) {
	reason = strings.Replace(reason, `"`, `'`, -1)
	err := APIError{false, reason}
	resp, _ := json.MarshalIndent(err, "", "    ")
	w.WriteHeader(errorCode)
	fmt.Fprint(w, string(resp))
}

func JSONErrorResponse(w http.ResponseWriter, errorCode int, err error) {
	w.WriteHeader(errorCode)
	fmt.Fprint(w, err.Error())
}

func RenderJSONOrStringError(w http.ResponseWriter, errorCode int, err error) {
	errStr := err.Error()
	var jsonObj map[string]interface{}
	if json.Unmarshal([]byte(errStr), &jsonObj) == nil {
		JSONErrorResponse(w, http.StatusInternalServerError, err)
		return
	}

	ErrorResponse(w, http.StatusInternalServerError, errStr)
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

func isNullJSON(jsonBytes []byte) bool {
	return string(jsonBytes) == "null"
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
}

func (i *jsonAPIHandler) POSTListing(w http.ResponseWriter, r *http.Request) {

	listingData, err := ioutil.ReadAll(r.Body)
	if err != nil {
		ErrorResponse(w, http.StatusConflict, err.Error())
		return
	}
	slug, err := i.node.CreateListing(listingData)
	if err != nil {
		if err == repo.ErrListingAlreadyExists {
			ErrorResponse(w, http.StatusConflict, "Listing already exists. Use PUT.")
			return
		}

		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	SanitizedResponse(w, fmt.Sprintf(`{"slug": "%s"}`, slug))
}

func (i *jsonAPIHandler) PUTListing(w http.ResponseWriter, r *http.Request) {
	listingData, err := ioutil.ReadAll(r.Body)
	if err != nil {
		ErrorResponse(w, http.StatusConflict, err.Error())
		return
	}
	err = i.node.UpdateListing(listingData, true)
	if err != nil {
		if err == repo.ErrListingDoesNotExist {
			ErrorResponse(w, http.StatusNotFound, "Listing not found.")
			return
		}

		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	SanitizedResponse(w, `{}`)
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
}

func (i *jsonAPIHandler) POSTPurchase(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	var data repo.PurchaseData
	err := decoder.Decode(&data)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	orderID, paymentAddr, amount, online, err := i.node.Purchase(&data)
	if err != nil {
		RenderJSONOrStringError(w, http.StatusInternalServerError, err)
		return
	}
	type purchaseReturn struct {
		PaymentAddress string              `json:"paymentAddress"`
		Amount         *repo.CurrencyValue `json:"amount"`
		VendorOnline   bool                `json:"vendorOnline"`
		OrderID        string              `json:"orderId"`
	}
	ret := purchaseReturn{paymentAddr, amount, online, orderID}
	b, err := json.MarshalIndent(ret, "", "    ")
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	SanitizedResponse(w, string(b))
}

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

func (i *jsonAPIHandler) POSTFollow(w http.ResponseWriter, r *http.Request) {
	type PeerID struct {
		ID string `json:"id"`
	}

	decoder := json.NewDecoder(r.Body)
	var pid PeerID
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
}

func (i *jsonAPIHandler) POSTUnfollow(w http.ResponseWriter, r *http.Request) {
	type PeerID struct {
		ID string `json:"id"`
	}
	decoder := json.NewDecoder(r.Body)
	var pid PeerID
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
}

var allCurrencyMapCache map[string]repo.CurrencyDefinition

func (i *jsonAPIHandler) GETWalletCurrencyDictionary(w http.ResponseWriter, r *http.Request) {
	var (
		resp      map[string]repo.CurrencyDefinition
		_, lookup = path.Split(r.URL.Path)
	)
	if lookup == "currencies" {
		if allCurrencyMapCache == nil {
			allCurrencyMapCache = repo.AllCurrencies().AsMap()
		}
		resp = allCurrencyMapCache
	} else {
		var upperLookup = strings.ToUpper(lookup)
		def, err := i.node.LookupCurrency(upperLookup)
		if err != nil {
			ErrorResponse(w, http.StatusNotFound, fmt.Sprintf("unknown definition for %s", lookup))
			return
		}
		resp = map[string]repo.CurrencyDefinition{upperLookup: def}
	}
	out, err := json.MarshalIndent(resp, "", "    ")
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	SanitizedResponse(w, string(out))
}

func (i *jsonAPIHandler) GETAddress(w http.ResponseWriter, r *http.Request) {
	_, coinType := path.Split(r.URL.Path)
	if coinType == "address" {
		ret := make(map[string]interface{})
		for ct, wal := range i.node.Multiwallet {
			ret[ct.CurrencyCode()] = wal.CurrentAddress(wallet.EXTERNAL).String()
		}
		out, err := json.MarshalIndent(ret, "", "    ")
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		SanitizedResponse(w, string(out))
		return
	}
	wal, err := i.node.Multiwallet.WalletForCurrencyCode(coinType)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, "Unknown wallet type")
		return
	}
	addr := wal.CurrentAddress(wallet.EXTERNAL)
	SanitizedResponse(w, fmt.Sprintf(`{"address": "%s"}`, addr.String()))
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
	_, coinType := path.Split(r.URL.Path)
	type balance struct {
		Confirmed   string                  `json:"confirmed"`
		Unconfirmed string                  `json:"unconfirmed"`
		Currency    repo.CurrencyDefinition `json:"currency"`
		Height      uint32                  `json:"height"`
	}
	if coinType == "balance" {
		ret := make(map[string]interface{})
		for ct, wal := range i.node.Multiwallet {
			height, _ := wal.ChainTip()
			defn, err := i.node.LookupCurrency(ct.CurrencyCode())
			if err != nil {
				ErrorResponse(w, http.StatusInternalServerError, err.Error())
				return
			}
			confirmed, unconfirmed := wal.Balance()
			ret[ct.CurrencyCode()] = balance{
				Confirmed:   confirmed.Value.String(),
				Unconfirmed: unconfirmed.Value.String(),
				Currency:    defn,
				Height:      height,
			}
		}
		out, err := json.MarshalIndent(ret, "", "    ")
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		SanitizedResponse(w, string(out))
		return
	}

	wal, err := i.node.Multiwallet.WalletForCurrencyCode(coinType)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, "unknown wallet type")
		return
	}
	height, _ := wal.ChainTip()
	confirmed, unconfirmed := wal.Balance()
	defn, err := i.node.LookupCurrency(coinType)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	bal := balance{
		Confirmed:   confirmed.Value.String(),
		Unconfirmed: unconfirmed.Value.String(),
		Currency:    defn,
		Height:      height,
	}
	out, err := json.MarshalIndent(bal, "", "    ")
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	SanitizedResponse(w, string(out))
}

func (i *jsonAPIHandler) POSTSpendCoinsForOrder(w http.ResponseWriter, r *http.Request) {
	var spendArgs core.SpendRequest
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&spendArgs)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	if spendArgs.OrderID == "" {
		ErrorResponse(w, http.StatusBadRequest, core.ErrOrderNotFound.Error())
		return
	}

	spendArgs.RequireAssociatedOrder = true
	result, err := i.node.Spend(&spendArgs)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	err = i.node.SendOrderPayment(result)
	if err != nil {
		log.Errorf("error sending order with id %s payment: %v", result.OrderID, err)
	}

	ser, err := json.MarshalIndent(result, "", "    ")
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	SanitizedResponse(w, string(ser))
}

func (i *jsonAPIHandler) POSTSpendCoins(w http.ResponseWriter, r *http.Request) {
	var spendArgs core.SpendRequest
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&spendArgs)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	result, err := i.node.Spend(&spendArgs)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	ser, err := json.MarshalIndent(result, "", "    ")
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	SanitizedResponse(w, string(ser))
}

func (i *jsonAPIHandler) GETConfig(w http.ResponseWriter, r *http.Request) {
	var usingTor bool
	if i.node.TorDialer != nil {
		usingTor = true
	}
	var wallets []string
	for coinType := range i.node.Multiwallet {
		wallets = append(wallets, coinType.CurrencyCode())
	}
	c := struct {
		PeerId  string   `json:"peerID"`
		Testnet bool     `json:"testnet"`
		Tor     bool     `json:"tor"`
		Wallets []string `json:"wallets"`
	}{
		PeerId:  i.node.IPFSIdentityString(),
		Testnet: i.node.TestNetworkEnabled(),
		Tor:     usingTor,
		Wallets: wallets,
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
	if err = i.node.ValidateMultiwalletHasPreferredCurrencies(settings); err != nil {
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
			i.node.Service.DisconnectFromPeer(id)
		}
		i.node.BanManager.SetBlockedIds(blockedIds)
	}
	if settings.StoreModerators != nil {
		modsToAdd, modsToDelete := extractModeratorChanges(*settings.StoreModerators, nil)
		go func(modsToAdd, modsToDelete []string) {
			if err := i.node.NotifyModerators(modsToAdd, modsToDelete); err != nil {
				log.Error(err)
			}
		}(modsToAdd, modsToDelete)
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
	if err = i.node.ValidateMultiwalletHasPreferredCurrencies(settings); err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	currentSettings, err := i.node.Datastore.Settings().Get()
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
			i.node.Service.DisconnectFromPeer(id)
		}
		i.node.BanManager.SetBlockedIds(blockedIds)
	}
	if settings.StoreModerators != nil {
		modsToAdd, modsToDelete := extractModeratorChanges(*settings.StoreModerators, currentSettings.StoreModerators)
		go func(modsToAdd, modsToDelete []string) {
			if err := i.node.NotifyModerators(modsToAdd, modsToDelete); err != nil {
				log.Error(err)
			}
		}(modsToAdd, modsToDelete)
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
}

func (i *jsonAPIHandler) GETSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := i.node.Datastore.Settings().Get()
	if err != nil {
		ErrorResponse(w, http.StatusNotFound, err.Error())
		return
	}
	settings.Version = &i.node.UserAgent
	settingsJSON, err := json.MarshalIndent(&settings, "", "    ")
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	SanitizedResponse(w, string(settingsJSON))
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
	currentSettings, err := i.node.Datastore.Settings().Get()
	if err != nil {
		ErrorResponse(w, http.StatusNotFound, "Settings is not yet set. Use POST.")
		return
	}
	if err = validateSMTPSettings(settings); err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	if err = i.node.ValidateMultiwalletHasPreferredCurrencies(settings); err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	err = i.node.Datastore.Settings().Update(settings)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
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
			i.node.Service.DisconnectFromPeer(id)
		}
		i.node.BanManager.SetBlockedIds(blockedIds)
	}
	if settings.StoreModerators != nil {
		modsToAdd, modsToDelete := extractModeratorChanges(*settings.StoreModerators, currentSettings.StoreModerators)
		if err := i.node.SetModeratorsOnListings(*settings.StoreModerators); err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		if err := i.node.SeedNode(); err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		go func(modsToAdd, modsToDelete []string) {
			if err := i.node.NotifyModerators(modsToAdd, modsToDelete); err != nil {
				log.Error(err)
			}
		}(modsToAdd, modsToDelete)
	}
	SanitizedResponse(w, `{}`)
}

func (i *jsonAPIHandler) GETClosestPeers(w http.ResponseWriter, r *http.Request) {
	_, peerID := path.Split(r.URL.Path)
	var peerIDs []string
	peers, err := ipfs.Query(i.node.DHT, peerID)
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

func (i *jsonAPIHandler) GETExchangeRate(w http.ResponseWriter, r *http.Request) {
	s := strings.Split(r.URL.Path, "/")
	var currencyCode, coinType string
	if len(s) <= 5 && len(s) > 3 {
		coinType = s[3]
	}
	if len(s) >= 5 {
		currencyCode = s[4]
	}
	wal, err := i.node.Multiwallet.WalletForCurrencyCode(coinType)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	if currencyCode == "" || strings.ToLower(currencyCode) == "exchangerate" {
		currencyMap, err := wal.ExchangeRates().GetAllRates(true)
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		exchangeRateJSON, err := json.MarshalIndent(currencyMap, "", "    ")
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		SanitizedResponse(w, string(exchangeRateJSON))

	} else {
		def, err := i.node.LookupCurrency(currencyCode)
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		rate, err := wal.ExchangeRates().GetExchangeRate(def.CurrencyCode().String())
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		fmt.Fprintf(w, `%.2f`, rate)
	}
}

func (i *jsonAPIHandler) GETFollowers(w http.ResponseWriter, r *http.Request) {
	_, peerID := path.Split(r.URL.Path)
	useCache, _ := strconv.ParseBool(r.URL.Query().Get("usecache"))
	if peerID == "" || strings.ToLower(peerID) == "followers" || peerID == i.node.IPFSIdentityString() {
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
		if isNullJSON(ret) {
			ret = []byte("[]")
		}
		SanitizedResponse(w, string(ret))
	} else {
		followBytes, err := ipfs.ResolveThenCat(i.node.IpfsNode, ipnspath.FromString(path.Join(peerID, "followers.json")), time.Minute, i.node.IPNSQuorumSize, useCache)
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
		if isNullJSON(ret) {
			ret = []byte("[]")
		}
		w.Header().Set("Cache-Control", "public, max-age=600, immutable")
		SanitizedResponse(w, string(ret))
	}
}

func (i *jsonAPIHandler) GETFollowing(w http.ResponseWriter, r *http.Request) {
	_, peerID := path.Split(r.URL.Path)
	useCache, _ := strconv.ParseBool(r.URL.Query().Get("usecache"))
	if peerID == "" || strings.ToLower(peerID) == "following" || peerID == i.node.IPFSIdentityString() {
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
		if isNullJSON(ret) {
			ret = []byte("[]")
		}
		SanitizedResponse(w, string(ret))
	} else {
		followBytes, err := ipfs.ResolveThenCat(i.node.IpfsNode, ipnspath.FromString(path.Join(peerID, "following.json")), time.Minute, i.node.IPNSQuorumSize, useCache)
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
	getPersonalInventory := peerIDString == "" || peerIDString == i.node.IPFSIdentityString()
	if getPersonalInventory {
		var (
			err       error
			inventory interface{}
		)

		if slug == "" {
			inventory, err = i.node.GetLocalInventory()
		} else {
			inventory, err = i.node.GetLocalInventoryForSlug(slug)
		}
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}

		ret, err := json.MarshalIndent(inventory, "", "    ")
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}

		if isNullJSON(ret) {
			fmt.Fprint(w, `[]`)
			return
		}
		SanitizedResponse(w, string(ret))
		return
	}

	// If we want another peer's inventory crawl IPFS with an optional cache
	var err error
	useCacheBool := false
	useCacheString := r.URL.Query().Get("usecache")
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
		Quantity string `json:"quantity"`
	}
	decoder := json.NewDecoder(r.Body)
	var invList []inv
	err := decoder.Decode(&invList)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	for _, in := range invList {
		q, ok := new(big.Int).SetString(in.Quantity, 10)
		if !ok {
			ErrorResponse(w, http.StatusBadRequest, "error parsing quantity")
			return
		}
		err = i.node.Datastore.Inventory().Put(in.Slug, in.Variant, q)
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
}

func (i *jsonAPIHandler) GETListings(w http.ResponseWriter, r *http.Request) {
	_, peerID := path.Split(r.URL.Path)
	useCache, _ := strconv.ParseBool(r.URL.Query().Get("usecache"))
	maxAge := r.URL.Query().Get("max-age")
	if maxAge == "" {
		maxAge = "600"
	} else {
		_, err := strconv.ParseUint(maxAge, 10, 32)
		if err != nil {
			ErrorResponse(w, http.StatusBadRequest, "max-age must be integer")
			return
		}
	}
	if peerID == "" || strings.ToLower(peerID) == "listings" || peerID == i.node.IPFSIdentityString() {
		listingsBytes, err := i.node.GetListings()
		if err != nil {
			ErrorResponse(w, http.StatusNotFound, err.Error())
			return
		}
		SanitizedResponse(w, string(listingsBytes))
	} else {
		listingsBytes, err := ipfs.ResolveThenCat(i.node.IpfsNode, ipnspath.FromString(path.Join(peerID, "listings.json")), time.Minute, i.node.IPNSQuorumSize, useCache)
		if err != nil {
			ErrorResponse(w, http.StatusNotFound, err.Error())
			return
		}
		normalizedIndex, err := repo.UnmarshalJSONSignedListingIndex(listingsBytes)
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, fmt.Sprintf("failed to parse listing index: %s", err.Error()))
			return
		}

		normalizedBytes, err := json.MarshalIndent(normalizedIndex, "", "    ")
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, fmt.Sprintf("failed to normalize listing index: %s", err.Error()))
			return
		}

		w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%s, immutable", maxAge))
		SanitizedResponse(w, string(normalizedBytes))
	}
}

func (i *jsonAPIHandler) GETListing(w http.ResponseWriter, r *http.Request) {
	urlPath, listingID := path.Split(r.URL.Path)
	_, peerID := path.Split(urlPath[:len(urlPath)-1])
	useCache, _ := strconv.ParseBool(r.URL.Query().Get("usecache"))
	if peerID == "" || strings.ToLower(peerID) == "listing" || peerID == i.node.IPFSIdentityString() {
		var sl *pb.SignedListing
		_, err := cid.Decode(listingID)
		if err == nil {
			sl, err = i.node.GetListingFromHash(listingID)
			if err != nil {
				ErrorResponse(w, http.StatusNotFound, "Listing not found.")
				return
			}
			sl.Hash = listingID
		} else {
			sl, err = i.node.GetListingFromSlug(listingID)
			if err != nil {
				ErrorResponse(w, http.StatusNotFound, "Listing not found.")
				return
			}
			hash, err := ipfs.GetHashOfFile(i.node.IpfsNode, path.Join(i.node.RepoPath, "root", "listings", listingID+".json"))
			if err != nil {
				ErrorResponse(w, http.StatusInternalServerError, err.Error())
				return
			}
			sl.Hash = hash
		}

		rsl := repo.NewSignedListingFromProtobuf(sl)

		if err := rsl.GetListing().UpdateCouponsFromDatastore(i.node.Datastore.Coupons()); err != nil {
			log.Warningf("updating coupons for listing (%s): %s", rsl.GetSlug(), err.Error())
		}

		if err := rsl.Normalize(); err != nil {
			ErrorResponse(w, http.StatusInternalServerError, fmt.Sprintf("normalizing listing: %s", err.Error()))
			return
		}

		out, err := rsl.MarshalJSON()
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		SanitizedResponseM(w, string(out), new(pb.SignedListing))
		return
	}

	var listingBytes []byte
	var hash string
	_, err := cid.Decode(listingID)
	if err == nil {
		listingBytes, err = ipfs.Cat(i.node.IpfsNode, listingID, time.Minute)
		if err != nil {
			ErrorResponse(w, http.StatusNotFound, err.Error())
			return
		}
		hash = listingID
		w.Header().Set("Cache-Control", "public, max-age=29030400, immutable")
	} else {
		listingBytes, err = ipfs.ResolveThenCat(i.node.IpfsNode, ipnspath.FromString(path.Join(peerID, "listings", listingID+".json")), time.Minute, i.node.IPNSQuorumSize, useCache)
		if err != nil {
			ErrorResponse(w, http.StatusNotFound, err.Error())
			return
		}
		hash, err = ipfs.GetHash(i.node.IpfsNode, bytes.NewReader(listingBytes))
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

	rsl := repo.NewSignedListingFromProtobuf(sl)

	if err := rsl.Normalize(); err != nil {
		ErrorResponse(w, http.StatusInternalServerError, fmt.Sprintf("normalizing listing: %s", err.Error()))
		return
	}

	out, err := rsl.MarshalJSON()
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	SanitizedResponseM(w, string(out), new(pb.SignedListing))
}

func (i *jsonAPIHandler) GETProfile(w http.ResponseWriter, r *http.Request) {
	_, peerID := path.Split(r.URL.Path)
	var profile pb.Profile
	var err error
	cacheBool := r.URL.Query().Get("usecache")
	useCache, _ := strconv.ParseBool(cacheBool)

	if peerID == "" || strings.ToLower(peerID) == "profile" || peerID == i.node.IPFSIdentityString() {
		profile, err = i.node.GetProfile()
		if err != nil && err == core.ErrorProfileNotFound {
			ErrorResponse(w, http.StatusNotFound, err.Error())
			return
		} else if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
	} else {
		profile, err = i.node.FetchProfile(peerID, useCache)
		if err != nil {
			ErrorResponse(w, http.StatusNotFound, err.Error())
			return
		}
		if profile.PeerID != peerID {
			ErrorResponse(w, http.StatusNotFound, "invalid profile: peer id mismatch on found profile")
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
	_, peerID := path.Split(r.URL.Path)
	SanitizedResponse(w, fmt.Sprintf(`{"followsMe": %t}`, i.node.Datastore.Followers().FollowsMe(peerID)))
}

func (i *jsonAPIHandler) GETIsFollowing(w http.ResponseWriter, r *http.Request) {
	_, peerID := path.Split(r.URL.Path)
	SanitizedResponse(w, fmt.Sprintf(`{"isFollowing": %t}`, i.node.Datastore.Following().IsFollowing(peerID)))
}

func (i *jsonAPIHandler) POSTOrderConfirmation(w http.ResponseWriter, r *http.Request) {
	type orderConf struct {
		OrderID string `json:"orderId"`
		Reject  bool   `json:"reject"`
	}
	decoder := json.NewDecoder(r.Body)
	var conf orderConf
	err := decoder.Decode(&conf)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	order, err := i.node.GetOrder(conf.OrderID)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	v5contract := order.Contract

	contract, state, funded, records, _, _, err := i.node.Datastore.Sales().GetByOrderId(conf.OrderID)
	if err != nil {
		ErrorResponse(w, http.StatusNotFound, err.Error())
		return
	}

	// TODO: Remove once broken contracts are migrated
	lookupCoin := v5contract.BuyerOrder.Payment.AmountCurrency.Code
	_, err = i.node.LookupCurrency(lookupCoin)
	if err != nil {
		log.Warningf("invalid BuyerOrder.Payment.Coin (%s) on order (%s)", lookupCoin, conf.OrderID)
		//contract.BuyerOrder.Payment.Coin = paymentCoin.String()
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
		err := i.node.ConfirmOfflineOrder(state, contract, records)
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
}

func (i *jsonAPIHandler) POSTOrderCancel(w http.ResponseWriter, r *http.Request) {
	type orderCancel struct {
		OrderID string `json:"orderId"`
	}
	decoder := json.NewDecoder(r.Body)
	var can orderCancel
	err := decoder.Decode(&can)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	contract, state, _, records, _, _, err := i.node.Datastore.Purchases().GetByOrderId(can.OrderID)
	if err != nil {
		ErrorResponse(w, http.StatusNotFound, "order not found")
		return
	}
	v5order, err := repo.ToV5Order(contract.BuyerOrder, nil)
	if err != nil {
		ErrorResponse(w, http.StatusNotFound, "order not found")
		return
	}

	// TODO: Remove once broken contracts are migrated
	lookupCoin := v5order.Payment.AmountCurrency.Code
	_, err = i.node.LookupCurrency(lookupCoin)
	if err != nil {
		log.Warningf("invalid BuyerOrder.Payment.Coin (%s) on order (%s)", lookupCoin, can.OrderID)
		//contract.BuyerOrder.Payment.Coin = paymentCoin.String()
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
}

func (i *jsonAPIHandler) POSTResyncBlockchain(w http.ResponseWriter, r *http.Request) {
	_, coinType := path.Split(r.URL.Path)
	creationDate, err := i.node.Datastore.Config().GetCreationDate()
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	if coinType == "resyncblockchain" {
		for _, wal := range i.node.Multiwallet {
			wal.ReSyncBlockchain(creationDate)
		}
		SanitizedResponse(w, `{}`)
		return
	}
	wal, err := i.node.Multiwallet.WalletForCurrencyCode(coinType)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, "Unknown wallet type")
		return
	}
	wal.ReSyncBlockchain(creationDate)
	SanitizedResponse(w, `{}`)
}

func (i *jsonAPIHandler) GETOrder(w http.ResponseWriter, r *http.Request) {
	_, orderID := path.Split(r.URL.Path)
	resp, err := i.node.GetOrder(orderID)
	if err != nil {
		ErrorResponse(w, http.StatusNotFound, "Order not found")
		return
	}

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

	SanitizedResponseM(w, out, new(pb.OrderRespApi))
}

func (i *jsonAPIHandler) POSTShutdown(w http.ResponseWriter, r *http.Request) {
	shutdown := func() {
		log.Info("OpenBazaar Server shutting down...")
		time.Sleep(time.Second)
		if core.Node != nil {
			core.Node.Datastore.Close()
			repoLockFile := filepath.Join(core.Node.RepoPath, fsrepo.LockFile)
			os.Remove(repoLockFile)
			core.Node.Multiwallet.Close()
			core.Node.IpfsNode.Close()
		}
		os.Exit(1)
	}
	go shutdown()
	SanitizedResponse(w, `{}`)
}

func (i *jsonAPIHandler) POSTRefund(w http.ResponseWriter, r *http.Request) {
	type orderCancel struct {
		OrderID string `json:"orderId"`
	}
	decoder := json.NewDecoder(r.Body)
	var can orderCancel
	err := decoder.Decode(&can)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	contract, state, _, records, _, _, err := i.node.Datastore.Sales().GetByOrderId(can.OrderID)
	if err != nil {
		ErrorResponse(w, http.StatusNotFound, "order not found")
		return
	}
	if state != pb.OrderState_AWAITING_FULFILLMENT && state != pb.OrderState_PARTIALLY_FULFILLED {
		ErrorResponse(w, http.StatusBadRequest, "order must be AWAITING_FULFILLMENT, or PARTIALLY_FULFILLED")
		return
	}

	// TODO: Remove once broken contracts are migrated
	order, err := i.node.GetOrder(can.OrderID)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	v5contract := order.Contract

	lookupCoin := v5contract.BuyerOrder.Payment.AmountCurrency.Code
	_, err = i.node.LookupCurrency(lookupCoin)
	if err != nil {
		log.Warningf("invalid BuyerOrder.Payment.Coin (%s) on order (%s)", lookupCoin, can.OrderID)
		//contract.BuyerOrder.Payment.Coin = paymentCoin.String()
	}

	err = i.node.RefundOrder(contract, records)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	SanitizedResponse(w, `{}`)
}

func (i *jsonAPIHandler) GETModerators(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("async")
	async, _ := strconv.ParseBool(query)
	include := r.URL.Query().Get("include")

	ctx := context.Background()
	if !async {
		removeDuplicates := func(xs []string) []string {
			found := make(map[string]bool)
			j := 0
			for i, x := range xs {
				if !found[x] {
					found[x] = true
					(xs)[j] = (xs)[i]
					j++
				}
			}
			return xs[:j]
		}
		peerInfoList, err := ipfs.FindPointers(i.node.DHT, ctx, core.ModeratorPointerID, 64)
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		var mods []string
		for _, p := range peerInfoList {
			id, err := ipfs.ExtractIDFromPointer(p)
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
					resp := &pb.PeerAndProfile{PeerId: m, Profile: &profile}
					mar := jsonpb.Marshaler{
						EnumsAsInts:  false,
						EmitDefaults: true,
						Indent:       "    ",
						OrigName:     false,
					}
					respJSON, err := mar.MarshalToString(resp)
					if err != nil {
						return
					}
					withProfiles = append(withProfiles, respJSON)
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
		id := r.URL.Query().Get("asyncID")
		if id == "" {
			idBytes := make([]byte, 16)
			_, err := rand.Read(idBytes)
			if err != nil {
				// TODO: if this happens, len(idBytes) != 16
				// how to handle this
				log.Error(err)
			}
			id = base58.Encode(idBytes)
		}

		type resp struct {
			ID string `json:"id"`
		}
		response := resp{id}
		respJSON, _ := json.MarshalIndent(response, "", "    ")
		w.WriteHeader(http.StatusAccepted)
		SanitizedResponse(w, string(respJSON))
		go func() {
			peerChan := ipfs.FindPointersAsync(i.node.DHT, ctx, core.ModeratorPointerID, 64)

			found := make(map[string]bool)
			foundMu := sync.Mutex{}
			for p := range peerChan {
				go func(pi ps.PeerInfo) {
					pid, err := ipfs.ExtractIDFromPointer(pi)
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
						resp := pb.PeerAndProfileWithID{Id: id, PeerId: pid, Profile: &profile}
						m := jsonpb.Marshaler{
							EnumsAsInts:  false,
							EmitDefaults: true,
							Indent:       "    ",
							OrigName:     false,
						}
						respJSON, err := m.MarshalToString(&resp)
						if err != nil {
							return
						}
						b, err := SanitizeProtobuf(respJSON, new(pb.PeerAndProfileWithID))
						if err != nil {
							return
						}
						i.node.Broadcast <- repo.PremarshalledNotifier{Payload: b}
					} else {
						type wsResp struct {
							ID     string `json:"id"`
							PeerID string `json:"peerId"`
						}
						resp := wsResp{id, pid}
						data, err := json.MarshalIndent(resp, "", "    ")
						if err != nil {
							return
						}
						i.node.Broadcast <- repo.PremarshalledNotifier{Payload: data}
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
	contract, state, _, records, _, _, err := i.node.Datastore.Sales().GetByOrderId(fulfill.OrderId)
	if err != nil {
		ErrorResponse(w, http.StatusNotFound, "order not found")
		return
	}

	// TODO: Remove once broken contracts are migrated
	order, err := i.node.GetOrder(fulfill.OrderId)
	if err != nil {
		ErrorResponse(w, http.StatusNotFound, "order not found")
		return
	}
	v5contract := order.Contract

	lookupCoin := v5contract.BuyerOrder.Payment.AmountCurrency.Code
	_, err = i.node.LookupCurrency(lookupCoin)
	if err != nil {
		log.Warningf("invalid BuyerOrder.Payment.Coin (%s) on order (%s)", lookupCoin, fulfill.OrderId)
		//contract.BuyerOrder.Payment.Coin = paymentCoin.String()
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
	contract, state, _, records, _, _, err := i.node.Datastore.Purchases().GetByOrderId(or.OrderID)
	if err != nil {
		ErrorResponse(w, http.StatusNotFound, "order not found")
		return
	}

	v5order, err := repo.ToV5Order(contract.BuyerOrder, nil)
	if err != nil {
		ErrorResponse(w, http.StatusNotFound, "order not found")
		return
	}

	// TODO: Remove once broken contracts are migrated
	lookupCoin := v5order.Payment.AmountCurrency.Code
	_, err = i.node.LookupCurrency(lookupCoin)
	if err != nil {
		log.Warningf("invalid BuyerOrder.Payment.Coin (%s) on order (%s)", lookupCoin, or.OrderID)
		//contract.BuyerOrder.Payment.Coin = paymentCoin.String()
	}

	if state != pb.OrderState_FULFILLED &&
		state != pb.OrderState_RESOLVED &&
		state != pb.OrderState_PAYMENT_FINALIZED {
		errorString := fmt.Sprintf("must be one of the following states to leave a rating and complete the order: %s, %s, %s",
			pb.OrderState_FULFILLED.String(),
			pb.OrderState_RESOLVED.String(),
			pb.OrderState_PAYMENT_FINALIZED.String(),
		)
		ErrorResponse(w, http.StatusBadRequest, errorString)
		return
	}

	if len(contract.VendorOrderFulfillment) == 0 && contract.BuyerOrder.Payment.Method == pb.Order_Payment_MODERATED {
		ErrorResponse(w, http.StatusBadRequest, "moderated orders can only be completed if the vendor has fulfilled the order")
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

	err = i.node.CompleteOrder(&or, contract, records)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	SanitizedResponse(w, `{}`)
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
	var (
		isSale   bool
		contract *pb.RicardianContract
		state    pb.OrderState
		records  []*wallet.TransactionRecord
		//paymentCoin *repo.CurrencyCode
	)
	contract, state, _, records, _, _, err = i.node.Datastore.Purchases().GetByOrderId(d.OrderID)
	if err != nil {
		contract, state, _, records, _, _, err = i.node.Datastore.Sales().GetByOrderId(d.OrderID)
		if err != nil {
			ErrorResponse(w, http.StatusNotFound, "Order not found")
			return
		}
		isSale = true
	}

	// TODO: Remove once broken contracts are migrated
	v5order, err := repo.ToV5Order(contract.BuyerOrder, nil)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	lookupCoin := v5order.Payment.AmountCurrency.Code
	_, err = i.node.LookupCurrency(lookupCoin)
	if err != nil {
		log.Warningf("invalid BuyerOrder.Payment.Coin (%s) on order (%s)", lookupCoin, d.OrderID)
		//contract.BuyerOrder.Payment.Coin = paymentCoin.String()
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
}

func (i *jsonAPIHandler) POSTCloseDispute(w http.ResponseWriter, r *http.Request) {
	type disputeParams struct {
		OrderID          string  `json:"orderId"`
		Resolution       string  `json:"resolution"`
		BuyerPercentage  float32 `json:"buyerPercentage"`
		VendorPercentage float32 `json:"vendorPercentage"`
	}
	decoder := json.NewDecoder(r.Body)
	var d disputeParams
	err := decoder.Decode(&d)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	disputeCase, err := i.node.Datastore.Cases().GetByCaseID(d.OrderID)
	if err != nil {
		ErrorResponse(w, http.StatusNotFound, err.Error())
	}

	err = i.node.CloseDispute(disputeCase.CaseID, d.BuyerPercentage, d.VendorPercentage, d.Resolution, disputeCase.PaymentCoin)
	if err != nil {
		switch err {
		case core.ErrCaseNotFound:
			ErrorResponse(w, http.StatusNotFound, err.Error())
		case core.ErrCloseFailureCaseExpired:
			ErrorResponse(w, http.StatusBadRequest, err.Error())
		default:
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	SanitizedResponse(w, `{}`)
}

func (i *jsonAPIHandler) GETCase(w http.ResponseWriter, r *http.Request) {
	_, orderID := path.Split(r.URL.Path)
	buyerContract, vendorContract, buyerErrors, vendorErrors, state, read, date, buyerOpened, claim, resolution, err := i.node.Datastore.Cases().GetCaseMetadata(orderID)
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

	if buyerContract.BuyerOrder.Payment.BigAmount == "" {
		v5order, err := repo.ToV5Order(buyerContract.BuyerOrder, nil)
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		buyerContract.BuyerOrder = v5order
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

	unread, err := i.node.Datastore.Chat().GetUnreadCount(orderID)
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

	err = i.node.Datastore.Cases().MarkAsRead(orderID)
	if err != nil {
		log.Error(err)
	}
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
	var (
		contract *pb.RicardianContract
		state    pb.OrderState
		records  []*wallet.TransactionRecord
		//paymentCoin *repo.CurrencyCode
	)
	contract, state, _, records, _, _, err = i.node.Datastore.Purchases().GetByOrderId(rel.OrderID)
	if err != nil {
		contract, state, _, records, _, _, err = i.node.Datastore.Sales().GetByOrderId(rel.OrderID)
		if err != nil {
			ErrorResponse(w, http.StatusNotFound, "Order not found")
			return
		}
	}

	v5order, err := repo.ToV5Order(contract.BuyerOrder, nil)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	// TODO: Remove once broken contracts are migrated
	lookupCoin := v5order.Payment.AmountCurrency.Code
	_, err = i.node.LookupCurrency(lookupCoin)
	if err != nil {
		log.Warningf("invalid BuyerOrder.Payment.Coin (%s) on order (%s)", lookupCoin, rel.OrderID)
		//contract.BuyerOrder.Payment.Coin = paymentCoin.String()
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
}

func (i *jsonAPIHandler) POSTReleaseEscrow(w http.ResponseWriter, r *http.Request) {
	var (
		rel struct {
			OrderID string `json:"orderId"`
		}
		contract *pb.RicardianContract
		state    pb.OrderState
		records  []*wallet.TransactionRecord
		//paymentCoin *repo.CurrencyCode
	)

	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&rel)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	contract, state, _, records, _, _, err = i.node.Datastore.Sales().GetByOrderId(rel.OrderID)
	if err != nil {
		ErrorResponse(w, http.StatusNotFound, "Order not found")
		return
	}

	// TODO: Remove once broken contracts are migrated
	order, err := i.node.GetOrder(rel.OrderID)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, "Could not retrieve the order")
		return
	}

	lookupCoin := order.Contract.BuyerOrder.Payment.AmountCurrency.Code
	_, err = i.node.LookupCurrency(lookupCoin)
	if err != nil {
		log.Warningf("invalid BuyerOrder.Payment.Coin (%s) on order (%s)", lookupCoin, rel.OrderID)
		//contract.BuyerOrder.Payment.Coin = paymentCoin.String()
	}

	if state != pb.OrderState_PENDING && state != pb.OrderState_FULFILLED && state != pb.OrderState_DISPUTED {
		ErrorResponse(w, http.StatusBadRequest, "Release escrow can only be called when sale is pending, fulfilled, or disputed")
		return
	}

	activeDispute, err := i.node.DisputeIsActive(contract)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	if activeDispute {
		ErrorResponse(w, http.StatusBadRequest, "Release escrow can only be called after dispute has expired")
		return
	}

	if !(&repo.SaleRecord{Contract: contract}).SupportsTimedEscrowRelease() {
		ErrorResponse(w, http.StatusBadRequest, "Escrowed currency does not support automatic release of funds to vendor")
		return
	}

	err = i.node.ReleaseFundsAfterTimeout(contract, records)
	if err != nil {
		switch err {
		case core.ErrPrematureReleaseOfTimedoutEscrowFunds:
			ErrorResponse(w, http.StatusUnauthorized, err.Error())
			return
		case core.EscrowTimeLockedError:
			ErrorResponse(w, http.StatusUnauthorized, err.Error())
			return
		default:
			ErrorResponse(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	err = i.node.SendFundsReleasedByVendor(contract.BuyerOrder.BuyerID.PeerID, contract.BuyerOrder.BuyerID.Pubkeys.Identity, rel.OrderID)
	if err != nil {
		log.Errorf("SendFundsReleasedByVendor error: %s", err.Error())
		log.Errorf("SendFundsReleasedByVendor: peerID: %s orderID: %s", contract.BuyerOrder.BuyerID.PeerID, rel.OrderID)
	}

	SanitizedResponse(w, `{}`)
}

func (i *jsonAPIHandler) POSTSignMessage(w http.ResponseWriter, r *http.Request) {
	type signRequest struct {
		Content string `json:"content"`
	}
	var (
		req signRequest
		err = json.NewDecoder(r.Body).Decode(&req)
	)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	sig, pubKey, err := core.SignPayload([]byte(req.Content), i.node.IpfsNode.PrivateKey)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	SanitizedResponse(w, fmt.Sprintf(`{"signature": "%s","pubkey":"%s","peerId":"%s"}`,
		hex.EncodeToString(sig),
		hex.EncodeToString(pubKey),
		i.node.IpfsNode.Identity.Pretty()))
}

func (i *jsonAPIHandler) POSTVerifyMessage(w http.ResponseWriter, r *http.Request) {
	type ciphertext struct {
		Content   string `json:"content"`
		Signature string `json:"signature"`
		Pubkey    string `json:"pubkey"`
		PeerId    string `json:"peerId"`
	}
	var msg ciphertext
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&msg)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	keyBytes, err := hex.DecodeString(msg.Pubkey)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	sigBytes, err := hex.DecodeString(msg.Signature)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	peerID, err := core.VerifyPayload([]byte(msg.Content), sigBytes, keyBytes)
	if err != nil {
		SanitizedResponse(w, `{"error":"VERIFICATION_FAILED"}`)
		return
	}

	if peerID != msg.PeerId {
		SanitizedResponse(w, `{"error":"PEER_ID_PUBKEY_MISMATCH"}`)
		return
	}
	SanitizedResponse(w, fmt.Sprintf(`{"error":"","peerId":"%s"}`, msg.PeerId))
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
	msgID, err := mh.Cast(encoded)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	chatPb := &pb.Chat{
		MessageId: msgID.B58String(),
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
		err = i.node.Datastore.Chat().Put(msgID.B58String(), chat.PeerId, chat.Subject, chat.Message, t, false, true)
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	SanitizedResponse(w, fmt.Sprintf(`{"messageId": "%s"}`, msgID.B58String()))
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
		ErrorResponse(w, http.StatusBadRequest, "Group chats must include a unique subject to be used as the group chat ID")
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
	msgID, err := mh.Cast(encoded)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	chatPb := &pb.Chat{
		MessageId: msgID.B58String(),
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
		err = i.node.Datastore.Chat().Put(msgID.B58String(), "", chat.Subject, chat.Message, t, false, true)
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	SanitizedResponse(w, fmt.Sprintf(`{"messageId": "%s"}`, msgID.B58String()))
}

func (i *jsonAPIHandler) GETChatMessages(w http.ResponseWriter, r *http.Request) {
	_, peerID := path.Split(r.URL.Path)
	if strings.ToLower(peerID) == "chatmessages" {
		peerID = ""
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
	offsetID := r.URL.Query().Get("offsetId")
	messages := i.node.Datastore.Chat().GetMessages(peerID, r.URL.Query().Get("subject"), offsetID, l)

	ret, err := json.MarshalIndent(messages, "", "    ")
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	if isNullJSON(ret) {
		ret = []byte("[]")
	}
	SanitizedResponse(w, string(ret))
}

func (i *jsonAPIHandler) GETChatConversations(w http.ResponseWriter, r *http.Request) {
	conversations := i.node.Datastore.Chat().GetConversations()
	ret, err := json.MarshalIndent(conversations, "", "    ")
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	if isNullJSON(ret) {
		ret = []byte("[]")
	}
	SanitizedResponse(w, string(ret))
}

func (i *jsonAPIHandler) POSTMarkChatAsRead(w http.ResponseWriter, r *http.Request) {
	_, peerID := path.Split(r.URL.Path)
	if strings.ToLower(peerID) == "markchatasread" {
		peerID = ""
	}
	subject := r.URL.Query().Get("subject")
	lastID, updated, err := i.node.Datastore.Chat().MarkAsRead(peerID, subject, false, "")
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	if updated && peerID != "" {
		chatPb := &pb.Chat{
			MessageId: lastID,
			Subject:   subject,
			Flag:      pb.Chat_READ,
		}
		err = i.node.SendChat(peerID, chatPb)
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	if subject != "" {
		go func(subject string) {
			err := i.node.Datastore.Purchases().MarkAsRead(subject)
			if err != nil {
				log.Error(err)
			}
			err = i.node.Datastore.Sales().MarkAsRead(subject)
			if err != nil {
				log.Error(err)
			}
			err = i.node.Datastore.Cases().MarkAsRead(subject)
			if err != nil {
				log.Error(err)
			}
		}(subject)
	}
	SanitizedResponse(w, `{}`)
}

func (i *jsonAPIHandler) DELETEChatMessage(w http.ResponseWriter, r *http.Request) {
	_, messageID := path.Split(r.URL.Path)
	err := i.node.Datastore.Chat().DeleteMessage(messageID)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	SanitizedResponse(w, `{}`)
}

func (i *jsonAPIHandler) DELETEChatConversation(w http.ResponseWriter, r *http.Request) {
	_, peerID := path.Split(r.URL.Path)
	err := i.node.Datastore.Chat().DeleteConversation(peerID)
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
	offsetID := r.URL.Query().Get("offsetId")
	filter := r.URL.Query().Get("filter")

	types := strings.Split(filter, ",")
	var filters []string
	for _, t := range types {
		if t != "" {
			filters = append(filters, t)
		}
	}

	type notifData struct {
		Unread        int               `json:"unread"`
		Total         int               `json:"total"`
		Notifications []json.RawMessage `json:"notifications"`
	}
	notifs, total, err := i.node.Datastore.Notifications().GetAll(offsetID, l, filters)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	unread, err := i.node.Datastore.Notifications().GetUnreadCount()
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	payload := notifData{unread, total, []json.RawMessage{}}
	for _, n := range notifs {
		data, err := n.Data()
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
		}
		payload.Notifications = append(payload.Notifications, data)
	}
	ret, err := json.MarshalIndent(payload, "", "    ")
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	retString := string(ret)
	if strings.Contains(retString, "null") {
		retString = strings.Replace(retString, "null", "[]", -1)
	}
	SanitizedResponse(w, retString)
}

func (i *jsonAPIHandler) POSTMarkNotificationAsRead(w http.ResponseWriter, r *http.Request) {
	_, notifID := path.Split(r.URL.Path)
	err := i.node.Datastore.Notifications().MarkAsRead(notifID)
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
	_, notifID := path.Split(r.URL.Path)
	err := i.node.Datastore.Notifications().Delete(notifID)
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

	api, err := coreapi.NewCoreAPI(i.node.IpfsNode)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	pth, err := iface.ParsePath("/ipfs/" + imageHash)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	nd, err := api.Unixfs().Get(ctx, pth)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	f, ok := nd.(files.File)
	if !ok {
		ErrorResponse(w, http.StatusInternalServerError, "Invalid type assertion")
		return
	}

	w.Header().Set("Cache-Control", "public, max-age=29030400, immutable")
	w.Header().Del("Content-Type")
	http.ServeContent(w, r, imageHash, time.Now(), f)
}

func (i *jsonAPIHandler) GETAvatar(w http.ResponseWriter, r *http.Request) {
	urlPath, size := path.Split(r.URL.Path)
	_, peerID := path.Split(urlPath[:len(urlPath)-1])

	cacheBool := r.URL.Query().Get("usecache")
	useCache, _ := strconv.ParseBool(cacheBool)

	dr, err := i.node.FetchAvatar(peerID, size, useCache)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=600, immutable")
	w.Header().Del("Content-Type")
	http.ServeContent(w, r, path.Join("ipns", peerID, "images", size, "avatar"), time.Now(), dr)
}

func (i *jsonAPIHandler) GETHeader(w http.ResponseWriter, r *http.Request) {
	urlPath, size := path.Split(r.URL.Path)
	_, peerID := path.Split(urlPath[:len(urlPath)-1])

	cacheBool := r.URL.Query().Get("usecache")
	useCache, _ := strconv.ParseBool(cacheBool)

	dr, err := i.node.FetchHeader(peerID, size, useCache)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=600, immutable")
	w.Header().Del("Content-Type")
	http.ServeContent(w, r, path.Join("ipns", peerID, "images", size, "header"), time.Now(), dr)
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
				obj := pb.PeerAndProfile{PeerId: pid, Profile: &pro}
				m := jsonpb.Marshaler{
					EnumsAsInts:  false,
					EmitDefaults: true,
					Indent:       "    ",
					OrigName:     false,
				}
				respJSON, err := m.MarshalToString(&obj)
				if err != nil {
					return
				}
				ret = append(ret, respJSON)
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
		id := r.URL.Query().Get("asyncID")
		if id == "" {
			idBytes := make([]byte, 16)
			_, err := rand.Read(idBytes)
			if err != nil {
				// TODO: if this happens, len(idBytes) != 16
				// how to handle this
				log.Error(err)
			}
			id = base58.Encode(idBytes)
		}

		type resp struct {
			ID string `json:"id"`
		}
		response := resp{id}
		respJSON, _ := json.MarshalIndent(response, "", "    ")
		w.WriteHeader(http.StatusAccepted)
		SanitizedResponse(w, string(respJSON))
		go func() {
			type profileError struct {
				ID     string `json:"id"`
				PeerID string `json:"peerID"`
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
						i.node.Broadcast <- repo.PremarshalledNotifier{Payload: ret}
					}

					pro, err := i.node.FetchProfile(pid, useCache)
					if err != nil {
						respondWithError("not found")
						return
					}
					obj := pb.PeerAndProfileWithID{Id: id, PeerId: pid, Profile: &pro}
					m := jsonpb.Marshaler{
						EnumsAsInts:  false,
						EmitDefaults: true,
						Indent:       "    ",
						OrigName:     false,
					}
					respJSON, err := m.MarshalToString(&obj)
					if err != nil {
						respondWithError("error Marshalling to JSON")
						return
					}
					b, err := SanitizeProtobuf(respJSON, new(pb.PeerAndProfileWithID))
					if err != nil {
						respondWithError("error Marshalling to JSON")
						return
					}
					i.node.Broadcast <- repo.PremarshalledNotifier{Payload: b}
				}(p)
			}
		}()
	}
}

func (i *jsonAPIHandler) GETTransactions(w http.ResponseWriter, r *http.Request) {
	_, coinType := path.Split(r.URL.Path)
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
		Txid          string        `json:"txid"`
		Value         string        `json:"value"`
		Address       string        `json:"address"`
		Status        string        `json:"status"`
		ErrorMessage  string        `json:"errorMessage"`
		Memo          string        `json:"memo"`
		Timestamp     *repo.APITime `json:"timestamp"`
		Confirmations int32         `json:"confirmations"`
		Height        int32         `json:"height"`
		OrderID       string        `json:"orderId"`
		Thumbnail     string        `json:"thumbnail"`
		CanBumpFee    bool          `json:"canBumpFee"`
	}
	wal, err := i.node.Multiwallet.WalletForCurrencyCode(coinType)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, "Unknown wallet type")
		return
	}
	transactions, err := wal.Transactions()
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	metadata, err := i.node.Datastore.TxMetadata().GetAll()
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	var txs []Tx
	passedOffset := false
	for i := len(transactions) - 1; i >= 0; i-- {
		t := transactions[i]
		tx := Tx{
			Txid:          t.Txid,
			Value:         t.Value,
			Timestamp:     repo.NewAPITime(t.Timestamp),
			Confirmations: int32(t.Confirmations),
			Height:        t.Height,
			Status:        string(t.Status),
			CanBumpFee:    true,
			ErrorMessage:  t.ErrorMessage,
		}
		m, ok := metadata[t.Txid]
		if ok {
			tx.Address = m.Address
			tx.Memo = m.Memo
			tx.OrderID = m.OrderId
			tx.Thumbnail = m.Thumbnail
			tx.CanBumpFee = m.CanBumpFee
		}
		if t.Status == wallet.StatusDead {
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
	if isNullJSON(ret) {
		ret = []byte("[]")
	}
	SanitizedResponse(w, string(ret))
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
	if isNullJSON(ret) {
		ret = []byte("[]")
	}
	SanitizedResponse(w, string(ret))
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
	if isNullJSON(ret) {
		ret = []byte("[]")
	}
	SanitizedResponse(w, string(ret))
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
	if isNullJSON(ret) {
		ret = []byte("[]")
	}
	SanitizedResponse(w, string(ret))
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
	if isNullJSON(ret) {
		ret = []byte("[]")
	}
	SanitizedResponse(w, string(ret))
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
	if isNullJSON(ret) {
		ret = []byte("[]")
	}
	SanitizedResponse(w, string(ret))
}

func (i *jsonAPIHandler) POSTBlockNode(w http.ResponseWriter, r *http.Request) {
	_, peerID := path.Split(r.URL.Path)
	settings, err := i.node.Datastore.Settings().Get()
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	var nodes []string
	if settings.BlockedNodes != nil {
		for _, pid := range *settings.BlockedNodes {
			if pid == peerID {
				fmt.Fprint(w, `{}`)
				return
			}
			nodes = append(nodes, pid)
		}
	}
	go func(nd *ipfscore.IpfsNode, peerID string, quorum uint) {
		err := ipfs.RemoveAll(nd, peerID, quorum)
		if err != nil {
			log.Error(err)
		}
	}(i.node.IpfsNode, peerID, i.node.IPNSQuorumSize)
	nodes = append(nodes, peerID)
	settings.BlockedNodes = &nodes
	if err := i.node.Datastore.Settings().Put(settings); err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	pid, err := peer.IDB58Decode(peerID)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	i.node.BanManager.AddBlockedId(pid)
	i.node.Service.DisconnectFromPeer(pid)
	SanitizedResponse(w, `{}`)
}

func (i *jsonAPIHandler) DELETEBlockNode(w http.ResponseWriter, r *http.Request) {
	_, peerID := path.Split(r.URL.Path)
	settings, err := i.node.Datastore.Settings().Get()
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	if settings.BlockedNodes != nil {
		var nodes []string
		for _, pid := range *settings.BlockedNodes {
			if pid != peerID {
				nodes = append(nodes, pid)
			}
		}
		settings.BlockedNodes = &nodes
	}
	if err := i.node.Datastore.Settings().Put(settings); err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	pid, err := peer.IDB58Decode(peerID)
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
	var wal wallet.Wallet
	for _, w := range i.node.Multiwallet {
		_, err := w.GetTransaction(*txHash)
		if err == nil {
			wal = w
			break
		}
	}
	if wal == nil {
		ErrorResponse(w, http.StatusBadRequest, "transaction not found in any wallet")
		return
	}
	newTxid, err := wal.BumpFee(*txHash)
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
		Txid               string              `json:"txid"`
		Amount             *repo.CurrencyValue `json:"amount"`
		ConfirmedBalance   *repo.CurrencyValue `json:"confirmedBalance"`
		UnconfirmedBalance *repo.CurrencyValue `json:"unconfirmedBalance"`
		Timestamp          *repo.APITime       `json:"timestamp"`
		Memo               string              `json:"memo"`
	}
	confirmed, unconfirmed := wal.Balance()
	defn, err := i.node.LookupCurrency(wal.CurrencyCode())
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	txn, err := wal.GetTransaction(*newTxid)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	amt0, _ := repo.NewCurrencyValue(txn.Value, defn)
	amt0.Amount = new(big.Int).Mod(amt0.Amount, big.NewInt(-1))

	t := repo.NewAPITime(txn.Timestamp)
	resp := &response{
		Txid:               newTxid.String(),
		ConfirmedBalance:   &repo.CurrencyValue{Currency: defn, Amount: &confirmed.Value},
		UnconfirmedBalance: &repo.CurrencyValue{Currency: defn, Amount: &unconfirmed.Value},
		Amount:             amt0,
		Timestamp:          t,
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
	_, coinType := path.Split(r.URL.Path)

	fl := r.URL.Query().Get("feeLevel")
	amt := r.URL.Query().Get("amount")
	amount, ok := new(big.Int).SetString(amt, 10) //strconv.Atoi(amt)
	if !ok {
		ErrorResponse(w, http.StatusBadRequest, "invalid amount")
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
	case "SUPER_ECONOMIC":
		feeLevel = wallet.SUPER_ECONOMIC
	default:
		ErrorResponse(w, http.StatusBadRequest, "Unknown feeLevel")
		return
	}

	wal, err := i.node.Multiwallet.WalletForCurrencyCode(coinType)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, "Unknown wallet type")
		return
	}

	fee, err := wal.EstimateSpendFee(*amount, feeLevel)
	if err != nil {
		switch {
		case err == wallet.ErrInsufficientFunds:
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

	defn, err := i.node.LookupCurrency(coinType)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	resp := &repo.CurrencyValue{Currency: defn, Amount: &fee}
	ser, err := json.MarshalIndent(resp, "", "    ")
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	SanitizedResponse(w, string(ser))
}

func (i *jsonAPIHandler) GETFees(w http.ResponseWriter, r *http.Request) {
	_, coinType := path.Split(r.URL.Path)
	type fees struct {
		Priority      *repo.CurrencyValue `json:"priority"`
		Normal        *repo.CurrencyValue `json:"normal"`
		Economic      *repo.CurrencyValue `json:"economic"`
		SuperEconomic *repo.CurrencyValue `json:"superEconomic"`
	}
	if coinType == "fees" {
		ret := make(map[string]interface{})
		for ct, wal := range i.node.Multiwallet {
			priority := wal.GetFeePerByte(wallet.PRIOIRTY)
			normal := wal.GetFeePerByte(wallet.NORMAL)
			economic := wal.GetFeePerByte(wallet.ECONOMIC)
			superEconomic := wal.GetFeePerByte(wallet.SUPER_ECONOMIC)
			defn, err := i.node.LookupCurrency(wal.CurrencyCode())
			if err != nil {
				ErrorResponse(w, http.StatusInternalServerError, err.Error())
				return
			}
			ret[ct.CurrencyCode()] = fees{
				Priority:      &repo.CurrencyValue{Currency: defn, Amount: &priority},
				Normal:        &repo.CurrencyValue{Currency: defn, Amount: &normal},
				Economic:      &repo.CurrencyValue{Currency: defn, Amount: &economic},
				SuperEconomic: &repo.CurrencyValue{Currency: defn, Amount: &superEconomic},
			}
		}
		out, err := json.MarshalIndent(ret, "", "    ")
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		SanitizedResponse(w, string(out))
		return
	}
	wal, err := i.node.Multiwallet.WalletForCurrencyCode(coinType)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, "Unknown wallet type")
		return
	}
	priority := wal.GetFeePerByte(wallet.PRIOIRTY)
	normal := wal.GetFeePerByte(wallet.NORMAL)
	economic := wal.GetFeePerByte(wallet.ECONOMIC)
	superEconomic := wal.GetFeePerByte(wallet.SUPER_ECONOMIC)
	defn, err := i.node.LookupCurrency(wal.CurrencyCode())
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	f := fees{
		Priority:      &repo.CurrencyValue{Currency: defn, Amount: &priority},
		Normal:        &repo.CurrencyValue{Currency: defn, Amount: &normal},
		Economic:      &repo.CurrencyValue{Currency: defn, Amount: &economic},
		SuperEconomic: &repo.CurrencyValue{Currency: defn, Amount: &superEconomic},
	}
	out, err := json.MarshalIndent(f, "", "    ")
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	SanitizedResponse(w, string(out))
}

func (i *jsonAPIHandler) POSTCheckoutBreakdown(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	var data repo.PurchaseData
	err := decoder.Decode(&data)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	cb, err := i.node.CheckoutBreakdown(&data)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	out, err := json.MarshalIndent(cb, "", "    ")
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	SanitizedResponse(w, string(out))
}

func (i *jsonAPIHandler) POSTEstimateTotal(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	var data repo.PurchaseData
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
	fmt.Fprintf(w, "%s", amount.String())
}

func (i *jsonAPIHandler) GETRatings(w http.ResponseWriter, r *http.Request) {
	urlPath, slug := path.Split(r.URL.Path)
	_, peerID := path.Split(urlPath[:len(urlPath)-1])
	useCache, _ := strconv.ParseBool(r.URL.Query().Get("usecache"))

	if peerID == "ratings" {
		peerID = slug
		slug = ""
	}

	var indexBytes []byte
	if peerID != i.node.IPFSIdentityString() {
		indexBytes, _ = ipfs.ResolveThenCat(i.node.IpfsNode, ipnspath.FromString(path.Join(peerID, "ratings.json")), time.Minute, i.node.IPNSQuorumSize, useCache)
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

	ratingBytes, err := ipfs.Cat(i.node.IpfsNode, ratingID, time.Minute)
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
				ratingBytes, err := ipfs.Cat(i.node.IpfsNode, rid, time.Minute)
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
				respJSON, err := m.MarshalToString(rating)
				if err != nil {
					return
				}
				ret = append(ret, respJSON)
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
		id := r.URL.Query().Get("asyncID")
		if id == "" {
			idBytes := make([]byte, 16)
			_, err := rand.Read(idBytes)
			if err != nil {
				return
			}
			id = base58.Encode(idBytes)
		}

		type resp struct {
			ID string `json:"id"`
		}
		response := resp{id}
		respJSON, _ := json.MarshalIndent(response, "", "    ")
		w.WriteHeader(http.StatusAccepted)
		SanitizedResponse(w, string(respJSON))
		for _, r := range rp {
			go func(rid string) {
				type ratingError struct {
					ID       string `json:"id"`
					RatingID string `json:"ratingId"`
					Error    string `json:"error"`
				}
				respondWithError := func(errorMsg string) {
					e := ratingError{id, rid, errorMsg}
					ret, err := json.MarshalIndent(e, "", "    ")
					if err != nil {
						return
					}
					i.node.Broadcast <- repo.PremarshalledNotifier{Payload: ret}
				}
				ratingBytes, err := ipfs.Cat(i.node.IpfsNode, rid, time.Minute)
				if err != nil {
					respondWithError("not Found")
					return
				}

				rating := new(pb.Rating)
				err = jsonpb.UnmarshalString(string(ratingBytes), rating)
				if err != nil {
					respondWithError("invalid rating")
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
					respondWithError("error marshalling rating")
					return
				}
				b, err := SanitizeProtobuf(out, new(pb.RatingWithID))
				if err != nil {
					respondWithError("error marshalling rating")
					return
				}
				i.node.Broadcast <- repo.PremarshalledNotifier{Payload: b}
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

	// TODO: add the import listings function call

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

	_, coinType := path.Split(r.URL.Path)
	type status struct {
		Height   uint32 `json:"height"`
		BestHash string `json:"bestHash"`
	}
	if coinType == "status" {
		ret := make(map[string]interface{})
		for ct, wal := range i.node.Multiwallet {
			height, hash := wal.ChainTip()
			ret[ct.CurrencyCode()] = status{height, hash}
		}
		out, err := json.MarshalIndent(ret, "", "    ")
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		SanitizedResponse(w, string(out))
		return
	}
	wal, err := i.node.Multiwallet.WalletForCurrencyCode(coinType)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, "Unknown wallet type")
		return
	}
	height, hash := wal.ChainTip()
	st := status{height, hash}
	out, err := json.MarshalIndent(st, "", "    ")
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	SanitizedResponse(w, string(out))
}

func (i *jsonAPIHandler) GETIPNS(w http.ResponseWriter, r *http.Request) {
	ipfsStore := i.node.IpfsNode.Repo.Datastore()
	_, peerID := path.Split(r.URL.Path)

	pid, err := peer.IDB58Decode(peerID)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	peerIPNSRecord, err := ipfs.GetCachedIPNSRecord(ipfsStore, pid)
	if err != nil { // No record in datastore
		ErrorResponse(w, http.StatusNotFound, err.Error())
		return
	}

	var keyBytes []byte
	pubkey := i.node.IpfsNode.Peerstore.PubKey(pid)
	if pubkey == nil || !pid.MatchesPublicKey(pubkey) {
		keyval, err := ipfs.GetCachedPubkey(ipfsStore, peerID)
		if err != nil {
			ErrorResponse(w, http.StatusNotFound, err.Error())
			return
		}
		keyBytes = keyval
	} else {
		keyBytes, err = pubkey.Bytes()
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	type KeyAndRecord struct {
		Pubkey string `json:"pubkey"`
		Record string `json:"record"`
	}
	peerIPNSBytes, err := ggproto.Marshal(peerIPNSRecord)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, fmt.Sprintf("marshaling IPNS record: %s", err.Error()))
		return
	}

	ret := KeyAndRecord{hex.EncodeToString(keyBytes), string(peerIPNSBytes)}
	retBytes, err := json.MarshalIndent(ret, "", "    ")
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	go func(nd *ipfscore.IpfsNode, pid peer.ID, timeout time.Duration, quorum uint, useCache bool) {
		_, err := ipfs.Resolve(nd, pid, timeout, quorum, useCache)
		if err != nil {
			log.Error(err)
		}
	}(i.node.IpfsNode, pid, time.Minute, i.node.IPNSQuorumSize, false)
	fmt.Fprint(w, string(retBytes))
}

func (i *jsonAPIHandler) GETResolveIPNS(w http.ResponseWriter, r *http.Request) {
	_, peerID := path.Split(r.URL.Path)
	if len(peerID) == 0 || peerID == "resolveipns" {
		peerID = i.node.IpfsNode.Identity.Pretty()
	}

	type respType struct {
		PeerID string `json:"peerid"`
		Record struct {
			Hex string `json:"hex"`
		} `json:"record"`
	}
	var response = respType{PeerID: peerID}

	if i.node.IpfsNode.Identity.Pretty() == peerID {
		rec, err := ipfs.GetCachedIPNSRecord(i.node.IpfsNode.Repo.Datastore(), i.node.IpfsNode.Identity)
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, fmt.Sprintf("retrieving self: %s", err))
			return
		}
		ipnsBytes, err := proto.Marshal(rec)
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, fmt.Sprintf("marshaling self: %s", err))
			return
		}
		response.Record.Hex = hex.EncodeToString(ipnsBytes)
		b, err := json.MarshalIndent(response, "", "    ")
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, fmt.Sprintf("marshal json error: %s", err))
			return
		}

		SanitizedResponse(w, string(b))
		return
	}

	pid, err := peer.IDB58Decode(peerID)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*180)
	_, err = routing.GetPublicKey(i.node.IpfsNode.Routing, ctx, pid)
	cancel()
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	ctx, cancel = context.WithTimeout(context.Background(), time.Second*180)
	ipnsBytes, err := i.node.IpfsNode.Routing.GetValue(ctx, ipns.RecordKey(pid))
	cancel()
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	response.Record.Hex = hex.EncodeToString(ipnsBytes)
	b, err := json.MarshalIndent(response, "", "    ")
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, fmt.Sprintf("marshal json error: %s", err))
		return
	}

	SanitizedResponse(w, string(b))
}

func (i *jsonAPIHandler) POSTTestEmailNotifications(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	var settings repo.SMTPSettings
	err := decoder.Decode(&settings)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	profile, err := i.node.GetProfile()
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	settings.OpenBazaarName = profile.Name
	notifier := smtpNotifier{&settings}
	err = notifier.notify(repo.TestNotification{})
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

// Enable bulk updating prices for your listings by percentage
func (i *jsonAPIHandler) POSTBulkUpdatePrices(w http.ResponseWriter, r *http.Request) {
	type BulkUpdatePriceRequest struct {
		Percentage float64 `json:"percentage"`
	}

	var bulkUpdate BulkUpdatePriceRequest
	err := json.NewDecoder(r.Body).Decode(&bulkUpdate)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	// Check for bad input
	if bulkUpdate.Percentage == 0 {
		SanitizedResponse(w, `{"success": "true"}`)
		return
	}

	log.Infof("Updating all listing prices by %v percent\n", bulkUpdate.Percentage)
	err = i.node.SetPriceOnListings(bulkUpdate.Percentage)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	SanitizedResponse(w, `{"success": "true"}`)
}

func (i *jsonAPIHandler) POSTBulkUpdateCurrency(w http.ResponseWriter, r *http.Request) {
	// Retrieve attribute and values to update
	type BulkUpdateRequest struct {
		Currencies []string `json:"currencies"`
	}

	var bulkUpdate BulkUpdateRequest
	err := json.NewDecoder(r.Body).Decode(&bulkUpdate)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	// Check for no currencies selected
	if len(bulkUpdate.Currencies) == 0 {
		SanitizedResponse(w, `{"success": "false", "reason":"No currencies specified"}`)
		return
	}

	log.Info("Updating currencies for all listings to: ", bulkUpdate.Currencies)
	err = i.node.SetCurrencyOnListings(bulkUpdate.Currencies)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	SanitizedResponse(w, `{"success": "true"}`)
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
		ld.Slug, err = i.node.GeneratePostSlug(ld.Status)
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
}

// GET a list of posts (self or peer)
func (i *jsonAPIHandler) GETPosts(w http.ResponseWriter, r *http.Request) {
	_, peerID := path.Split(r.URL.Path)
	useCache, _ := strconv.ParseBool(r.URL.Query().Get("usecache"))
	if peerID == "" || strings.ToLower(peerID) == "posts" || peerID == i.node.IPFSIdentityString() {
		postsBytes, err := i.node.GetPosts()
		if err != nil {
			ErrorResponse(w, http.StatusNotFound, err.Error())
			return
		}
		SanitizedResponse(w, string(postsBytes))
	} else {
		postsBytes, err := ipfs.ResolveThenCat(i.node.IpfsNode, ipnspath.FromString(path.Join(peerID, "posts.json")), time.Minute, i.node.IPNSQuorumSize, useCache)
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
	urlPath, postID := path.Split(r.URL.Path)
	_, peerID := path.Split(urlPath[:len(urlPath)-1])
	useCache, _ := strconv.ParseBool(r.URL.Query().Get("usecache"))
	m := jsonpb.Marshaler{
		EnumsAsInts:  false,
		EmitDefaults: false,
		Indent:       "    ",
		OrigName:     false,
	}
	if peerID == "" || strings.ToLower(peerID) == "post" || peerID == i.node.IPFSIdentityString() {
		var sl *pb.SignedPost
		_, err := cid.Decode(postID)
		if err == nil {
			sl, err = i.node.GetPostFromHash(postID)
			if err != nil {
				ErrorResponse(w, http.StatusNotFound, "Post not found.")
				return
			}
			sl.Hash = postID
		} else {
			sl, err = i.node.GetPostFromSlug(postID)
			if err != nil {
				ErrorResponse(w, http.StatusNotFound, "Post not found.")
				return
			}
			hash, err := ipfs.GetHashOfFile(i.node.IpfsNode, path.Join(i.node.RepoPath, "root", "posts", postID+".json"))
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
		SanitizedResponseM(w, out, new(pb.SignedPost))
		return
	}

	var postBytes []byte
	var hash string
	_, err := cid.Decode(postID)
	if err == nil {
		postBytes, err = ipfs.Cat(i.node.IpfsNode, postID, time.Minute)
		if err != nil {
			ErrorResponse(w, http.StatusNotFound, err.Error())
			return
		}
		hash = postID
		w.Header().Set("Cache-Control", "public, max-age=29030400, immutable")
	} else {
		postBytes, err = ipfs.ResolveThenCat(i.node.IpfsNode, ipnspath.FromString(path.Join(peerID, "posts", postID+".json")), time.Minute, i.node.IPNSQuorumSize, useCache)
		if err != nil {
			ErrorResponse(w, http.StatusNotFound, err.Error())
			return
		}
		hash, err = ipfs.GetHash(i.node.IpfsNode, bytes.NewReader(postBytes))
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

// POSTSendOrderMessage - used to manually send an order message
func (i *jsonAPIHandler) POSTResendOrderMessage(w http.ResponseWriter, r *http.Request) {
	type sendRequest struct {
		OrderID     string `json:"orderID"`
		MessageType string `json:"messageType"`
	}

	var args sendRequest
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&args)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	if args.MessageType == "" {
		ErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("missing messageType argument"))
		return
	}
	if args.OrderID == "" {
		ErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("missing orderID argument"))
		return
	}

	msgInt, ok := pb.Message_MessageType_value[strings.ToUpper(args.MessageType)]
	if !ok {
		ErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("unknown messageType (%s)", args.MessageType))
		return
	}

	if err := i.node.ResendCachedOrderMessage(args.OrderID, pb.Message_MessageType(msgInt)); err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	SanitizedResponse(w, `{}`)
}

// GETScanOfflineMessages - used to manually trigger offline message scan
func (i *jsonAPIHandler) GETScanOfflineMessages(w http.ResponseWriter, r *http.Request) {
	if lastManualScan.IsZero() {
		lastManualScan = time.Now()
	} else {
		if time.Since(lastManualScan) >= OfflineMessageScanInterval {
			i.node.MessageRetriever.RunOnce()
			lastManualScan = time.Now()
		}
	}
	SanitizedResponse(w, `{}`)
}

func (i *jsonAPIHandler) POSTHashMessage(w http.ResponseWriter, r *http.Request) {
	type hashRequest struct {
		Content string `json:"content"`
	}
	var (
		req hashRequest
		err = json.NewDecoder(r.Body).Decode(&req)
	)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	messageHash, err := ipfs.EncodeMultihash([]byte(req.Content))
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	SanitizedResponse(w, fmt.Sprintf(`{"hash": "%s"}`,
		messageHash.B58String()))
}
