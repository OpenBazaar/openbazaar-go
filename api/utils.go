package api

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"gx/ipfs/QmXauCuJzmzapetmC6W4TuDJLL1yFFrVzSHoWv8YdbmnxH/go-libp2p-peerstore"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/OpenBazaar/jsonpb"
	"github.com/OpenBazaar/openbazaar-go/repo"

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

func configureAPIAuthentication(r *http.Request, w http.ResponseWriter, username string, password string, cookieValue string) {
	if username == "" || password == "" {
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
		if !ok || username != username || strings.ToLower(password) != strings.ToLower(password) {
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprint(w, "403 - Forbidden")
			return
		}
	}
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
