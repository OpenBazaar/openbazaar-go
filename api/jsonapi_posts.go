package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"

	"github.com/OpenBazaar/jsonpb"
	"github.com/OpenBazaar/openbazaar-go/core"
	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/golang/protobuf/ptypes"
	ipnspath "github.com/ipfs/go-ipfs/path"
)

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
		pid, err := i.node.NameSystem.Resolve(context.Background(), peerID)
		if err != nil {
			ErrorResponse(w, http.StatusNotFound, err.Error())
			return
		}
		peerID = pid.Pretty()
		postsBytes, err := i.node.IPNSResolveThenCat(ipnspath.FromString(path.Join(peerID, "posts.json")), time.Minute, useCache)
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

		out, _ := getJSONOutput(m, w, sl)
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
		pid, err := i.node.NameSystem.Resolve(context.Background(), peerID)
		if err != nil {
			ErrorResponse(w, http.StatusNotFound, err.Error())
			return
		}
		peerID = pid.Pretty()
		postBytes, err = i.node.IPNSResolveThenCat(ipnspath.FromString(path.Join(peerID, "posts", postID+".json")), time.Minute, useCache)
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
	out, _ := getJSONOutput(m, w, sl)
	SanitizedResponseM(w, out, new(pb.SignedPost))
}

func (i *jsonAPIHandler) POSTPurchase(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	var data core.PurchaseData
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
		PaymentAddress string `json:"paymentAddress"`
		Amount         uint64 `json:"amount"`
		VendorOnline   bool   `json:"vendorOnline"`
		OrderID        string `json:"orderId"`
	}
	ret := purchaseReturn{paymentAddr, amount, online, orderID}
	b, err := json.MarshalIndent(ret, "", "    ")
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	SanitizedResponse(w, string(b))
}
