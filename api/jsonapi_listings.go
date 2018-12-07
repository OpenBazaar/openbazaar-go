package api

import (
	"context"
	"encoding/json"
	"fmt"
	"gx/ipfs/QmZoWKhxUmZ2seW4BzX6fJkNR8hh9PsGModr7q171yq2SS/go-libp2p-peer"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/OpenBazaar/jsonpb"
	"github.com/OpenBazaar/openbazaar-go/core"
	"github.com/OpenBazaar/openbazaar-go/pb"
	ipnspath "github.com/ipfs/go-ipfs/path"
)

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
		Quantity int64  `json:"quantity"`
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
		pid, err := i.node.NameSystem.Resolve(context.Background(), peerID)
		if err != nil {
			ErrorResponse(w, http.StatusNotFound, err.Error())
			return
		}
		peerID = pid.Pretty()
		listingsBytes, err := i.node.IPNSResolveThenCat(ipnspath.FromString(path.Join(peerID, "listings.json")), time.Minute, useCache)
		if err != nil {
			ErrorResponse(w, http.StatusNotFound, err.Error())
			return
		}
		w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%s, immutable", maxAge))
		SanitizedResponse(w, string(listingsBytes))
	}
}

func (i *jsonAPIHandler) GETListing(w http.ResponseWriter, r *http.Request) {
	var sl *pb.SignedListing

	urlPath, listingID := path.Split(r.URL.Path)
	_, peerID := path.Split(urlPath[:len(urlPath)-1])
	useCache, _ := strconv.ParseBool(r.URL.Query().Get("usecache"))
	m := jsonpb.Marshaler{
		EnumsAsInts:  false,
		EmitDefaults: false,
		Indent:       "    ",
		OrigName:     false,
	}

	// Retrieve local listing
	if peerID == "" || strings.ToLower(peerID) == "listing" || peerID == i.node.IPFSIdentityString() {

		sl, err := getSignedListing(w, i.node, listingID)
		if err != nil {
			return
		}

		replaceCouponHashesWithPlaintext(w, i.node.Datastore.Coupons(), sl)
		setAdditionalItemPrices(sl)

		out, _ := getJSONOutput(m, w, sl)

		SanitizedResponseM(w, out, new(pb.SignedListing))
		return
	}

	// Retrieve listing from the OpenBazaar network
	sl, _ = getSignedListingFromNetwork(w, i.node, listingID, peerID, useCache)

	out, _ := getJSONOutput(m, w, sl)
	SanitizedResponseM(w, out, new(pb.SignedListing))
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
