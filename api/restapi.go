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
	"github.com/ipfs/go-ipfs/commands"
	"github.com/ipfs/go-ipfs/core/corehttp"
	"github.com/ipfs/go-ipfs/core"
	"github.com/OpenBazaar/openbazaar-go/ipfs"
)

type RestAPIConfig struct {
	Headers      map[string][]string
	BlockList    *corehttp.BlockList
	Writable     bool
	PathPrefixes []string
}

type restAPIHandler struct {
	node   *core.IpfsNode
	config RestAPIConfig
	path string
	context commands.Context
	rootHash string
}

func newRestAPIHandler(node *core.IpfsNode, ctx commands.Context) (*restAPIHandler, error) {

	// Add the current node directory in case it's note already added.
	dirHash, aerr := ipfs.AddDirectory(ctx, path.Join(ctx.ConfigRoot, "node"))
	if aerr != nil {
		log.Error(aerr)
		return nil, aerr
	}

	prefixes := []string{"/ob/"}
	i := &restAPIHandler{
		node:   node,
		config: RestAPIConfig{
			Writable:     true,
			BlockList:    &corehttp.BlockList{},
			PathPrefixes: prefixes,
		},
		path: ctx.ConfigRoot,
		context: ctx,
		rootHash: dirHash,
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
	f, err := os.Create(path.Join(i.path, "node", "profile"))
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
	hash, aerr := ipfs.AddDirectory(i.context, path.Join(i.path, "node"))
	if aerr != nil {
		fmt.Fprintf(w, `{"success": false, "reason": %s}`, aerr)
		return
	}
	_, perr := ipfs.Publish(i.context, hash)
	if perr != nil {
		fmt.Fprintf(w, `{"success": false, "reason": %s}`, perr)
		return
	}
	if hash != i.rootHash {
		if err := ipfs.UnPinDir(i.context, i.rootHash); err != nil {
			fmt.Fprintf(w, `{"success": false, "reason": %s}`, err)
			return
		}
		i.rootHash = hash
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
	imgPath := path.Join(i.path, "node", "avatar")
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

	hash, aerr := ipfs.AddDirectory(i.context, path.Join(i.path, "node"))
	if aerr != nil {
		fmt.Fprintf(w, `{"success": false, "reason": %s}`, aerr)
		return
	}

	_, perr := ipfs.Publish(i.context, hash)
	if perr != nil {
		fmt.Fprintf(w, `{"success": false, "reason": %s}`, perr)
		return
	}
	if hash != i.rootHash {
		if err := ipfs.UnPinDir(i.context, i.rootHash); err != nil {
			fmt.Fprintf(w, `{"success": false, "reason": %s}`, err)
			return
		}
		i.rootHash = hash
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
	imgPath := path.Join(i.path, "node", "header")
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

	hash, aerr := ipfs.AddDirectory(i.context, path.Join(i.path, "node"))
	if aerr != nil {
		fmt.Fprintf(w, `{"success": false, "reason": %s}`, aerr)
		return
	}

	_, perr := ipfs.Publish(i.context, hash)
	if perr != nil {
		fmt.Fprintf(w, `{"success": false, "reason": %s}`, perr)
		return
	}
	if hash != i.rootHash {
		if err := ipfs.UnPinDir(i.context, i.rootHash); err != nil {
			fmt.Fprintf(w, `{"success": false, "reason": %s}`, err)
			return
		}
		i.rootHash = hash
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
		if err := os.MkdirAll(path.Join(i.path, "node", "listings", img.Directory), os.ModePerm); err != nil {
			fmt.Fprintf(w, `{"success": false, "reason": %s}`, err)
			return
		}
		imgPath := path.Join(i.path, "node", "listings", img.Directory, img.Filename)
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
		hash, aerr := ipfs.AddFile(i.context, imgPath)
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


	c := new(pb.RicardianContract)
	if err := jsonpb.Unmarshal(r.Body, c); err != nil {
		errstr := err.Error()
		if errstr != "json: cannot unmarshal string into Go value of type pb.CountryCode" {
			fmt.Fprintf(w, `{"success": false, "reason": %s}`, err)
			return
		}
	}
	listingPath:= path.Join(i.path, "node", "listings", c.VendorListing.ListingName)
	if err := os.MkdirAll(listingPath, os.ModePerm); err != nil {
		fmt.Fprintf(w, `{"success": false, "reason": %s}`, err)
		return
	}

	// TODO: Add vendor identity to contract
	// TODO: Sign contract before dumping to disk
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
	out, err := m.MarshalToString(c)
	if err != nil {
		fmt.Fprintf(w, `{"success": false, "reason": %s}`, err)
		return
	}

	if _, err := f.WriteString(out); err != nil {
		fmt.Fprintf(w, `{"success": false, "reason": %s}`, err)
		return
	}

	hash, aerr := ipfs.AddDirectory(i.context, path.Join(i.path, "node"))
	if aerr != nil {
		fmt.Fprintf(w, `{"success": false, "reason": %s}`, aerr)
		return
	}

	_, perr := ipfs.Publish(i.context, hash)
	if perr != nil {
		fmt.Fprintf(w, `{"success": false, "reason": %s}`, perr)
		return
	}
	if hash != i.rootHash {
		if err := ipfs.UnPinDir(i.context, i.rootHash); err != nil {
			fmt.Fprintf(w, `{"success": false, "reason": %s}`, err)
			return
		}
		i.rootHash = hash
	}

	fmt.Fprintf(w, `{"success": true}`)
}

