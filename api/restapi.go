package api

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/OpenBazaar/openbazaar-go/core"
	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/golang/protobuf/jsonpb"
	"github.com/ipfs/go-ipfs/core/corehttp"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path"
	"runtime/debug"
	"strings"
)

type RestAPIConfig struct {
	Headers      map[string][]string
	BlockList    *corehttp.BlockList
	Writable     bool
	PathPrefixes []string
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

	prefixes := []string{"/ob/"}
	i := &restAPIHandler{
		config: RestAPIConfig{
			Writable:     true,
			BlockList:    &corehttp.BlockList{},
			PathPrefixes: prefixes,
		},
		node: node,
	}
	return i, nil
}

// TODO: Build out the api
func (i *restAPIHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "PUT,POST,DELETE")
	w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")

	// Stop here if its Preflighted OPTIONS request
	if r.Method == "OPTIONS" {
		return
	}
	dump, err := httputil.DumpRequest(r, false)
	if err != nil {
		log.Errorf("Error reading http request: ", err)
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
	if i.config.Writable {
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
			fmt.Fprint(w, "delete")
			return
		}
	}

	if r.Method == "GET" {
		get(i, u.String(), w, r)
		return
	}
}

func (i *restAPIHandler) PUTProfile(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	f, err := os.Create(path.Join(i.node.RepoPath, "root", "profile"))
	if err != nil {
		fmt.Fprintf(w, `{"success": false, "reason": %s}`, err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			panic(err)
		}
	}()

	dec := json.NewDecoder(r.Body)
	for {
		var v map[string]interface{}
		err := dec.Decode(&v)
		if err == io.EOF {
			break
		}
		b, err := json.MarshalIndent(v, "", "    ")
		if err != nil {
			fmt.Fprintf(w, `{"success": false, "reason": %s}`, err)
			return
		}
		if _, err := f.WriteString(string(b)); err != nil {
			fmt.Fprintf(w, `{"success": false, "reason": %s}`, err)
			return
		}
	}
	if err := i.node.SeedNode(); err != nil {
		fmt.Fprintf(w, `{"success": false, "reason": %s}`, err)
		return
	}
	fmt.Fprintf(w, `{"success": true}`)
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
		fmt.Fprintf(w, `{"success": false, "reason": %s}`, err)
		return
	}
	imgPath := path.Join(i.node.RepoPath, "root", "avatar")
	out, err := os.Create(imgPath)
	if err != nil {
		fmt.Fprintf(w, `{"success": false, "reason": %s}`, err)
		return
	}

	dec := base64.NewDecoder(base64.StdEncoding, strings.NewReader(data.Avatar))

	defer out.Close()

	_, err = io.Copy(out, dec)
	if err != nil {
		fmt.Fprintf(w, `{"success": false, "reason": %s}`, err)
		return
	}

	if err := i.node.SeedNode(); err != nil {
		fmt.Fprintf(w, `{"success": false, "reason": %s}`, err)
		return
	}

	fmt.Fprint(w, `{"success": true}`)
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
		fmt.Fprintf(w, `{"success": false, "reason": %s}`, err)
		return
	}
	imgPath := path.Join(i.node.RepoPath, "root", "header")
	out, err := os.Create(imgPath)
	if err != nil {
		fmt.Fprintf(w, `{"success": false, "reason": %s}`, err)
		return
	}

	dec := base64.NewDecoder(base64.StdEncoding, strings.NewReader(data.Header))

	defer out.Close()

	_, err = io.Copy(out, dec)
	if err != nil {
		fmt.Fprintf(w, `{"success": false, "reason": %s}`, err)
		return
	}

	if err := i.node.SeedNode(); err != nil {
		fmt.Fprintf(w, `{"success": false, "reason": %s}`, err)
		return
	}

	fmt.Fprint(w, `{"success": true}`)
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
		fmt.Fprintf(w, `{"success": false, "reason": %s}`, err)
		return
	}
	var imageHashes []string
	for _, img := range images {
		if err := os.MkdirAll(path.Join(i.node.RepoPath, "root", "listings", img.Directory), os.ModePerm); err != nil {
			fmt.Fprintf(w, `{"success": false, "reason": %s}`, err)
			return
		}
		imgPath := path.Join(i.node.RepoPath, "root", "listings", img.Directory, img.Filename)
		out, err := os.Create(imgPath)
		if err != nil {
			fmt.Fprintf(w, `{"success": false, "reason": %s}`, err)
			return
		}

		dec := base64.NewDecoder(base64.StdEncoding, strings.NewReader(img.Image))

		defer out.Close()

		_, err = io.Copy(out, dec)
		if err != nil {
			fmt.Fprintf(w, `{"success": false, "reason": %s}`, err)
			return
		}
		hash, aerr := ipfs.AddFile(i.node.Context, imgPath)
		if aerr != nil {
			fmt.Fprintf(w, `{"success": false, "reason": %s}`, aerr)
			return
		}
		imageHashes = append(imageHashes, hash+" "+img.Filename)
	}
	jsonHashes, err := json.Marshal(imageHashes)
	if err != nil {
		fmt.Fprintf(w, `{"success": false, "reason": %s}`, err)
		return
	}
	fmt.Fprintf(w, `{"success": true, hashes: %s}`, string(jsonHashes))
}

func (i *restAPIHandler) POSTListing(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")

	l := new(pb.Listing)
	jsonpb.Unmarshal(r.Body, l)
	listingPath := path.Join(i.node.RepoPath, "root", "listings", l.ListingName)
	if err := os.MkdirAll(listingPath, os.ModePerm); err != nil {
		fmt.Fprintf(w, `{"success": false, "reason": %s}`, err)
		return
	}

	contract, err := i.node.SignListing(l)
	if err != nil {
		fmt.Fprintf(w, `{"success": false, "reason": %s}`, err)
		return
	}
	f, err := os.Create(path.Join(listingPath, "listing.json"))
	if err != nil {
		fmt.Fprintf(w, `{"success": false, "reason": %s}`, err)
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
		fmt.Fprintf(w, `{"success": false, "reason": %s}`, err)
		return
	}

	if _, err := f.WriteString(out); err != nil {
		fmt.Fprintf(w, `{"success": false, "reason": %s}`, err)
		return
	}
	err = i.node.UpdateListingIndex(contract)
	if err != nil {
		fmt.Fprintf(w, `{"success": false, "reason": %s}`, err)
		return
	}

	if err := i.node.SeedNode(); err != nil {
		fmt.Fprintf(w, `{"success": false, "reason": %s}`, err)
		return
	}

	fmt.Fprintf(w, `{"success": true}`)
}
func (i *restAPIHandler) POSTPurchase(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")

	decoder := json.NewDecoder(r.Body)
	var data core.PurchaseData
	err := decoder.Decode(&data)
	if err != nil {
		fmt.Fprintf(w, `{"success": false, "reason": %s}`, err)
		return
	}
	if err := i.node.Purchase(&data); err != nil {
		fmt.Fprintf(w, `{"success": false, "reason": %s}`, err)
		return
	}
	fmt.Fprintf(w, `{"success": true}`)
}

func (i *restAPIHandler) GETStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	_, peerId := path.Split(r.URL.Path)
	status := i.node.GetPeerStatus(peerId)
	fmt.Fprintf(w, `{"status": %s}`, status)
}

func (i *restAPIHandler) GETPeers(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	peers, err := ipfs.ConnectedPeers(i.node.Context)
	if err != nil {
		fmt.Fprintf(w, `{"success": false, "reason": %s}`, err)
		return
	}

	peerJson, err := json.MarshalIndent(peers, "", "    ")
	if err != nil {
		fmt.Fprintf(w, `{"success": false, "reason": %s}`, err)
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
		fmt.Fprintf(w, `{"success": false, "reason": %s}`, err)
		return
	}
	if err := i.node.Follow(pid.ID); err != nil {
		fmt.Fprintf(w, `{"success": false, "reason": %s}`, err)
		return
	}
	fmt.Fprintf(w, `{"success": true}`)
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
		fmt.Fprintf(w, `{"success": false, "reason": %s}`, err)
		return
	}
	if err := i.node.Unfollow(pid.ID); err != nil {
		fmt.Fprintf(w, `{"success": false, "reason": %s}`, err)
		return
	}
	fmt.Fprintf(w, `{"success": true}`)
}
