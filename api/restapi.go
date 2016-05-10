package api

import (
	"io"
		"path"
	"net/http"
	"fmt"
	"runtime/debug"
	"net/url"
	"os"
	"strings"
	"encoding/json"
	"encoding/base64"
	"net/http/httputil"
	"github.com/golang/protobuf/jsonpb"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/ipfs/go-ipfs/core/corehttp"
	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/OpenBazaar/openbazaar-go/core"
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
	dirHash, aerr := ipfs.AddDirectory(node.Context, path.Join(node.RepoPath, "node"))
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
		fmt.Fprint(w, "get")
		return
	}
}

func (i *restAPIHandler) PUTProfile (w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	f, err := os.Create(path.Join(i.node.RepoPath, "node", "profile"))
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

func (i *restAPIHandler) PUTAvatar (w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	type ImgData struct{
		Avatar string
	}
	decoder := json.NewDecoder(r.Body)
	data := new(ImgData)
	err := decoder.Decode(&data)

	if err != nil{
		fmt.Fprintf(w, `{"success": false, "reason": %s}`, err)
		return
	}
	imgPath := path.Join(i.node.RepoPath, "node", "avatar")
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

func (i *restAPIHandler) PUTHeader (w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	type ImgData struct{
		Header string
	}
	decoder := json.NewDecoder(r.Body)
	data := new(ImgData)
	err := decoder.Decode(&data)

	if err != nil{
		fmt.Fprintf(w, `{"success": false, "reason": %s}`, err)
		return
	}
	imgPath := path.Join(i.node.RepoPath, "node", "header")
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

func (i *restAPIHandler) PUTImage (w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	type ImgData struct{
		Directory string
		Filename string
		Image string
	}
	decoder := json.NewDecoder(r.Body)
	var images []ImgData
	err := decoder.Decode(&images)
	if err != nil{
		fmt.Fprintf(w, `{"success": false, "reason": %s}`, err)
		return
	}
	var imageHashes []string
	for _, img := range(images) {
		if err := os.MkdirAll(path.Join(i.node.RepoPath, "node", "listings", img.Directory), os.ModePerm); err != nil {
			fmt.Fprintf(w, `{"success": false, "reason": %s}`, err)
			return
		}
		imgPath := path.Join(i.node.RepoPath, "node", "listings", img.Directory, img.Filename)
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
		imageHashes = append(imageHashes, hash + " " + img.Filename)
	}
	jsonHashes, err := json.Marshal(imageHashes)
	if err != nil {
		fmt.Fprintf(w, `{"success": false, "reason": %s}`, err)
		return
	}
	fmt.Fprintf(w, `{"success": true, hashes: %s}`, string(jsonHashes))
}

func (i *restAPIHandler) POSTContract (w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")


	l := new(pb.Listing)
	if err := jsonpb.Unmarshal(r.Body, l); err != nil {
		errstr := err.Error()
		if errstr != "json: cannot unmarshal string into Go value of type pb.CountryCode" {
			fmt.Fprintf(w, `{"success": false, "reason": %s}`, err)
			return
		}
	}
	listingPath:= path.Join(i.node.RepoPath, "node", "listings", l.ListingName)
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

	m := jsonpb.Marshaler {
		EnumsAsInts: false,
		EmitDefaults: false,
		Indent: "    ",
		OrigName: false,
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

	if err := i.node.SeedNode(); err != nil {
		fmt.Fprintf(w, `{"success": false, "reason": %s}`, err)
		return
	}

	fmt.Fprintf(w, `{"success": true}`)
}

