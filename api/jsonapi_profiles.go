package api

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"gx/ipfs/QmZoWKhxUmZ2seW4BzX6fJkNR8hh9PsGModr7q171yq2SS/go-libp2p-peer"

	"github.com/OpenBazaar/openbazaar-go/ipfs"

	routing "gx/ipfs/QmRaVcGchmC1stHHK7YhcgEuTk5k1JiGS568pfYWMgT91H/go-libp2p-kad-dht"

	"github.com/OpenBazaar/jsonpb"
	"github.com/OpenBazaar/openbazaar-go/core"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/btcsuite/btcutil/base58"
	"github.com/ipfs/go-ipfs/core/coreunix"
	ipnspath "github.com/ipfs/go-ipfs/path"
)

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
		pid, err := i.node.NameSystem.Resolve(context.Background(), peerID)
		if err != nil {
			ErrorResponse(w, http.StatusNotFound, err.Error())
			return
		}
		peerID = pid.Pretty()
		profile, err = i.node.FetchProfile(peerID, useCache)
		if err != nil {
			ErrorResponse(w, http.StatusNotFound, err.Error())
			return
		}
		if profile.PeerID != peerID {
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
	out, _ := getJSONOutput(m, w, &profile)
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
		pid, err := i.node.NameSystem.Resolve(context.Background(), peerID)
		if err != nil {
			ErrorResponse(w, http.StatusNotFound, err.Error())
			return
		}
		peerID = pid.Pretty()
		followBytes, err := i.node.IPNSResolveThenCat(ipnspath.FromString(path.Join(peerID, "followers.json")), time.Minute, useCache)
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
		pid, err := i.node.NameSystem.Resolve(context.Background(), peerID)
		if err != nil {
			ErrorResponse(w, http.StatusNotFound, err.Error())
			return
		}
		peerID = pid.Pretty()
		followBytes, err := i.node.IPNSResolveThenCat(ipnspath.FromString(path.Join(peerID, "following.json")), time.Minute, useCache)
		if err != nil {
			ErrorResponse(w, http.StatusNotFound, err.Error())
			return
		}
		w.Header().Set("Cache-Control", "public, max-age=600, immutable")
		SanitizedResponse(w, string(followBytes))
	}
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
	out, _ := getJSONOutput(m, w, profile)
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
	out, _ := getJSONOutput(m, w, profile)
	SanitizedResponseM(w, out, new(pb.Profile))
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
	defer dr.Close()
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
	defer dr.Close()
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

		SanitizedResponse(w, prepareOutput(ret))
	} else {
		id := r.URL.Query().Get("asyncID")
		if id == "" {
			idBytes := make([]byte, 16)
			rand.Read(idBytes)
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
						i.node.Broadcast <- repo.PremarshalledNotifier{ret}
					}

					pro, err := i.node.FetchProfile(pid, useCache)
					if err != nil {
						respondWithError("Not found")
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
						respondWithError("Error Marshalling to JSON")
						return
					}
					b, err := SanitizeProtobuf(respJSON, new(pb.PeerAndProfileWithID))
					if err != nil {
						respondWithError("Error Marshalling to JSON")
						return
					}
					i.node.Broadcast <- repo.PremarshalledNotifier{b}
				}(p)
			}
		}()
	}
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
	go ipfs.RemoveAll(i.node.IpfsNode, peerID)
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
		indexBytes, _ = i.node.IPNSResolveThenCat(ipnspath.FromString(path.Join(peerID, "ratings.json")), time.Minute, useCache)

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

		SanitizedResponse(w, prepareOutput(ret))
	} else {
		id := r.URL.Query().Get("asyncID")
		if id == "" {
			idBytes := make([]byte, 16)
			rand.Read(idBytes)
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
					e := ratingError{id, rid, "Not found"}
					ret, err := json.MarshalIndent(e, "", "    ")
					if err != nil {
						return
					}
					i.node.Broadcast <- repo.PremarshalledNotifier{ret}
				}
				ratingBytes, err := ipfs.Cat(i.node.IpfsNode, rid, time.Minute)
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
				i.node.Broadcast <- repo.PremarshalledNotifier{b}
			}(r)
		}
	}
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
	_, err = i.node.Datastore.Settings().Get()
	if err != nil {
		ErrorResponse(w, http.StatusNotFound, "Settings is not yet set. Use POST.")
		return
	}

	setBannedNodes(settings, i.node.BanManager)

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
	if err = validateSMTPSettings(settings); err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	if err = i.node.ValidateMultiwalletHasPreferredCurrencies(settings); err != nil {
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

	setBannedNodes(settings, i.node.BanManager)

	err = i.node.Datastore.Settings().Update(settings)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	SanitizedResponse(w, `{}`)
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

	setBannedNodes(settings, i.node.BanManager)

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

func (i *jsonAPIHandler) GETModerators(w http.ResponseWriter, r *http.Request) {
	var moderatorsResponse string

	query := r.URL.Query().Get("async")
	async, _ := strconv.ParseBool(query)
	include := r.URL.Query().Get("include")

	withProfile := strings.ToLower(include) == "profile"

	ctx := context.Background()
	if !async {
		peerInfoList, err := ipfs.FindPointers(i.node.IpfsNode.Routing.(*routing.IpfsDHT), ctx, core.ModeratorPointerID, 64)
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}

		mods := getModeratorsFromPeerList(peerInfoList)

		if withProfile {
			var withProfiles []string
			withProfiles, _ = i.node.FetchPeersAndProfiles(mods, false)
			moderatorsResponse = formatProfiles(withProfiles)
		} else {
			res, err := json.MarshalIndent(mods, "", "    ")
			if err != nil {
				ErrorResponse(w, http.StatusInternalServerError, err.Error())
				return
			}
			moderatorsResponse = string(res)
		}
		if moderatorsResponse == "null" {
			moderatorsResponse = "[]"
		}
		SanitizedResponse(w, moderatorsResponse)
	} else {

		// Send back ID for this asynchronous request for tracking
		id := r.URL.Query().Get("asyncID")
		if id == "" {
			id = generateRandomID()
		}

		type resp struct {
			ID string `json:"id"`
		}
		response := resp{id}
		respJSON, _ := json.MarshalIndent(response, "", "    ")
		w.WriteHeader(http.StatusAccepted)
		SanitizedResponse(w, string(respJSON))

		// Retrieve profiles and return as they are discovered
		go func() {
			peerChan := ipfs.FindPointersAsync(i.node.IpfsNode.Routing.(*routing.IpfsDHT), ctx, core.ModeratorPointerID, 64)

			for p := range peerChan {
				retrieveProfileAsync(i.node, id, p, withProfile)
			}
		}()
	}
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
