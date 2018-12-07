package api

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"gx/ipfs/QmXauCuJzmzapetmC6W4TuDJLL1yFFrVzSHoWv8YdbmnxH/go-libp2p-peerstore"
	"gx/ipfs/QmZoWKhxUmZ2seW4BzX6fJkNR8hh9PsGModr7q171yq2SS/go-libp2p-peer"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/OpenBazaar/openbazaar-go/net"

	"gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"

	"github.com/OpenBazaar/jsonpb"
	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/golang/protobuf/proto"
	ipnspath "github.com/ipfs/go-ipfs/path"

	ps "gx/ipfs/QmXauCuJzmzapetmC6W4TuDJLL1yFFrVzSHoWv8YdbmnxH/go-libp2p-peerstore"

	"github.com/OpenBazaar/openbazaar-go/core"
	"github.com/btcsuite/btcutil/base58"

	"github.com/OpenBazaar/openbazaar-go/pb"
)

type TransactionQuery struct {
	OrderStates     []int    `json:"states"`
	SearchTerm      string   `json:"search"`
	SortByAscending bool     `json:"sortByAscending"`
	SortByRead      bool     `json:"sortByRead"`
	Limit           int      `json:"limit"`
	Exclude         []string `json:"exclude"`
}

func parseSearchTerms(q url.Values) (orderStates []pb.OrderState, searchTerm string, sortByAscending, sortByRead bool, limit int, err error) {
	limitStr := q.Get("limit")
	if limitStr == "" {
		limitStr = "-1"
	}
	limit, err = strconv.Atoi(limitStr)
	if err != nil {
		return orderStates, searchTerm, false, false, 0, err
	}
	stateQuery := q.Get("state")
	states := strings.Split(stateQuery, ",")
	for _, s := range states {
		if s != "" {
			i, err := strconv.Atoi(s)
			if err != nil {
				return orderStates, searchTerm, false, false, 0, err
			}
			orderStates = append(orderStates, pb.OrderState(i))
		}
	}
	searchTerm = q.Get("search")
	sortTerms := strings.Split(q.Get("sortBy"), ",")
	if len(sortTerms) > 0 {
		for _, term := range sortTerms {
			switch strings.ToLower(term) {
			case "data-asc":
				sortByAscending = true
			case "read":
				sortByRead = true
			}
		}
	}
	return orderStates, searchTerm, sortByAscending, sortByRead, limit, nil
}

func convertOrderStates(states []int) []pb.OrderState {
	var orderStates []pb.OrderState
	for _, i := range states {
		orderStates = append(orderStates, pb.OrderState(i))
	}
	return orderStates
}

func getModeratorsFromPeerList(peerInfoList []peerstore.PeerInfo) []string {
	var mods []string
	for _, p := range peerInfoList {
		id, err := core.ExtractIDFromPointer(p)
		if err != nil {
			continue
		}
		mods = append(mods, id)
	}
	return removeDuplicates(mods)
}

func removeDuplicates(xs []string) []string {
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

func formatProfiles(profiles []string) string {
	resp := "[\n"
	max := len(profiles)
	for i, r := range profiles {
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
	return resp
}

func generateRandomID() string {
	idBytes := make([]byte, 16)
	rand.Read(idBytes)
	return base58.Encode(idBytes)
}

func writeToAPILog(r *http.Request) {
	r.Header.Del("Cookie")
	r.Header.Del("Authorization")
	dump, err := httputil.DumpRequest(r, false)
	if err != nil {
		log.Error("Error reading http request:", err)
	}
	log.Debugf("%s", dump)
}

func checkForUnauthorizedIP(w http.ResponseWriter, allowed map[string]bool, remote string) {
	if len(allowed) > 0 {
		remoteAddr := strings.Split(remote, ":")
		if !allowed[remoteAddr[0]] {
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprint(w, "403 - Forbidden")
			return
		}
	}
}

func enableCORS(w http.ResponseWriter, cors *string) {
	w.Header().Set("Access-Control-Allow-Origin", *cors)
	w.Header().Set("Access-Control-Allow-Methods", "PUT,POST,PATCH,DELETE,GET,OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
}

func configureAPIAuthentication(r *http.Request, w http.ResponseWriter, configUsername string, configPassword string, cookieValue string) {
	if configUsername == "" || configPassword == "" {
		cookie, err := r.Cookie("OpenBazaar_Auth_Cookie")
		if err != nil {
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprint(w, "403 - Forbidden")
			return
		}
		if cookieValue != cookie.Value {
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
		if !ok || configUsername != username || strings.ToLower(password) != strings.ToLower(configPassword) {
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprint(w, "403 - Forbidden")
			return
		}
	}
}

func getJSONOutput(m jsonpb.Marshaler, w http.ResponseWriter, msg proto.Message) (string, error) {
	out, err := m.MarshalToString(msg)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return "", err
	}
	return out, nil
}

func setAdditionalItemPrices(sl *pb.SignedListing) {
	if sl.Listing.Metadata != nil && sl.Listing.Metadata.Version == 1 {
		for _, so := range sl.Listing.ShippingOptions {
			for _, ser := range so.Services {
				ser.AdditionalItemPrice = ser.Price
			}
		}
	}
}

func replaceCouponHashesWithPlaintext(w http.ResponseWriter, coupons repo.CouponStore, listing *pb.SignedListing) {
	savedCoupons, err := coupons.Get(listing.Listing.Slug)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	for _, coupon := range listing.Listing.Coupons {
		for _, c := range savedCoupons {
			if coupon.GetHash() == c.Hash {
				coupon.Code = &pb.Listing_Coupon_DiscountCode{c.Code}
				break
			}
		}
	}
}

func getSignedListingFromNetwork(w http.ResponseWriter, node *core.OpenBazaarNode, listingID string, peerID string, useCache bool) (*pb.SignedListing, error) {
	var listingBytes []byte
	var hash string
	_, err := cid.Decode(listingID)
	if err == nil {
		listingBytes, err = ipfs.Cat(node.IpfsNode, listingID, time.Minute)
		if err != nil {
			ErrorResponse(w, http.StatusNotFound, err.Error())
			return nil, err
		}
		hash = listingID
		w.Header().Set("Cache-Control", "public, max-age=29030400, immutable")
	} else {
		pid, err := node.NameSystem.Resolve(context.Background(), peerID)
		if err != nil {
			ErrorResponse(w, http.StatusNotFound, err.Error())
			return nil, err
		}
		peerID = pid.Pretty()
		listingBytes, err = node.IPNSResolveThenCat(ipnspath.FromString(path.Join(peerID, "listings", listingID+".json")), time.Minute, useCache)
		if err != nil {
			ErrorResponse(w, http.StatusNotFound, err.Error())
			return nil, err
		}
		hash, err = ipfs.GetHash(node.IpfsNode, bytes.NewReader(listingBytes))
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return nil, err
		}
		w.Header().Set("Cache-Control", "public, max-age=600, immutable")
	}
	sl := new(pb.SignedListing)
	err = jsonpb.UnmarshalString(string(listingBytes), sl)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return nil, err
	}
	sl.Hash = hash
	return sl, nil
}

func getSignedListing(w http.ResponseWriter, node *core.OpenBazaarNode, listingID string) (*pb.SignedListing, error) {
	var sl *pb.SignedListing
	_, err := cid.Decode(listingID)
	if err == nil {
		sl, err = node.GetListingFromHash(listingID)
		if err != nil {
			ErrorResponse(w, http.StatusNotFound, "Listing not found.")
			return nil, err
		}
		sl.Hash = listingID
	} else {
		sl, err = node.GetListingFromSlug(listingID)
		if err != nil {
			ErrorResponse(w, http.StatusNotFound, "Listing not found.")
			return nil, err
		}
		hash, err := ipfs.GetHashOfFile(node.IpfsNode, path.Join(node.RepoPath, "root", "listings", listingID+".json"))
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return nil, err
		}
		sl.Hash = hash
	}
	return sl, nil
}

func retrieveProfileAsync(node *core.OpenBazaarNode, requestId string, peer ps.PeerInfo, withProfile bool) {
	found := make(map[string]bool)
	foundMu := sync.Mutex{}

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

		if withProfile {
			profile, err := node.FetchProfile(pid, false)
			if err != nil {
				return
			}
			resp := pb.PeerAndProfileWithID{Id: requestId, PeerId: pid, Profile: &profile}
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
			node.Broadcast <- repo.PremarshalledNotifier{b}
		} else {
			type wsResp struct {
				ID     string `json:"id"`
				PeerID string `json:"peerId"`
			}
			resp := wsResp{requestId, pid}
			data, err := json.MarshalIndent(resp, "", "    ")
			if err != nil {
				return
			}
			node.Broadcast <- repo.PremarshalledNotifier{data}
		}
	}(peer)
}

func getTradeRecords(r *http.Request, w http.ResponseWriter, node *core.OpenBazaarNode, typeOfRecord string) (string, error) {
	var query TransactionQuery
	var ret []byte

	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&query)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return "", err
	}

	switch typeOfRecord {
	case "cases":
		cases, queryCount, err := node.Datastore.Cases().GetAll(convertOrderStates(query.OrderStates), query.SearchTerm, query.SortByAscending, query.SortByRead, query.Limit, query.Exclude)
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return "", err
		}
		for n, c := range cases {
			unread, err := node.Datastore.Chat().GetUnreadCount(c.CaseId)
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
		ret, err = json.MarshalIndent(cr, "", "    ")
	case "sales":
		sales, queryCount, err := node.Datastore.Sales().GetAll(convertOrderStates(query.OrderStates), query.SearchTerm, query.SortByAscending, query.SortByRead, query.Limit, query.Exclude)
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return "", err
		}
		for n, s := range sales {
			unread, err := node.Datastore.Chat().GetUnreadCount(s.OrderId)
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

		ret, err = json.MarshalIndent(sr, "", "    ")
	case "purchases":
		purchases, queryCount, err := node.Datastore.Purchases().GetAll(convertOrderStates(query.OrderStates), query.SearchTerm, query.SortByAscending, query.SortByRead, query.Limit, query.Exclude)
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return "", err
		}
		for n, p := range purchases {
			unread, err := node.Datastore.Chat().GetUnreadCount(p.OrderId)
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
		ret, err = json.MarshalIndent(pr, "", "    ")
	}

	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return "", err
	}
	if isNullJSON(ret) {
		ret = []byte("[]")
	}
	return string(ret), nil
}

func setBannedNodes(settings repo.SettingsData, manager *net.BanManager) {
	if settings.BlockedNodes != nil {
		var blockedIds []peer.ID
		for _, pid := range *settings.BlockedNodes {
			id, err := peer.IDB58Decode(pid)
			if err != nil {
				continue
			}
			blockedIds = append(blockedIds, id)
		}
		manager.SetBlockedIds(blockedIds)
	}
}

func prepareOutput(results []string) string {
	resp := "[\n"
	max := len(results)
	for i, r := range results {
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
	return resp
}
