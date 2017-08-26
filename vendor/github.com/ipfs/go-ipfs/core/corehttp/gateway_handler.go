package corehttp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	gopath "path"
	"runtime/debug"
	"strings"
	"time"

	core "github.com/ipfs/go-ipfs/core"
	coreapi "github.com/ipfs/go-ipfs/core/coreapi"
	coreiface "github.com/ipfs/go-ipfs/core/coreapi/interface"
	"github.com/ipfs/go-ipfs/importer"
	chunk "github.com/ipfs/go-ipfs/importer/chunk"
	dag "github.com/ipfs/go-ipfs/merkledag"
	"github.com/ipfs/go-ipfs/namesys"
	"github.com/ipfs/go-ipfs/routing/dht"
	path "github.com/ipfs/go-ipfs/path"
	proto "gx/ipfs/QmZ4Qi3GaRbjcx28Sme5eMH7RQjGkt8wHxt2a65oLaeFEV/gogo-protobuf/proto"
	recpb "gx/ipfs/QmWYCqr6UDqqD1bfRybaAPtbAqcN3TSJpveaBXMwbQ3ePZ/go-libp2p-record/pb"

	namepb "github.com/ipfs/go-ipfs/namesys/pb"
	"github.com/ipfs/go-ipfs/thirdparty/ds-help"
	"crypto/sha256"
	"encoding/hex"
	dhtpb "gx/ipfs/QmRmroYSdievxnjiuy99C8BzShNstdEWcEF3LQHF7fUbez/go-libp2p-kad-dht/pb"
	uio "github.com/ipfs/go-ipfs/unixfs/io"

	routing "gx/ipfs/QmNdaQ8itUU9jEZUwTsG4gHMaPmRfi6FEe89QjQAFbep3M/go-libp2p-routing"
	humanize "gx/ipfs/QmPSBJL4momYnE7DcUyk2DVhD6rH488ZmHBGLbxNdhU44K/go-humanize"
	cid "gx/ipfs/QmYhQaCYEcaPPjxJX7YcPcVKkQfRy6sJ7B3XmGFk82XYdQ/go-cid"
	node "gx/ipfs/Qmb3Hm9QDFmfYuET4pu7Kyg8JV78jFa1nvZx5vnCZsK4ck/go-ipld-format"
	multibase "gx/ipfs/QmcxkxTVuURV2Ptse8TvkqH5BQDwV62X1x19JqqvbBzwUM/go-multibase"
	"io/ioutil"
)

const (
	ipfsPathPrefix = "/ipfs/"
	ipnsPathPrefix = "/ipns/"
)

// gatewayHandler is a HTTP handler that serves IPFS objects (accessible by default at /ipfs/<path>)
// (it serves requests like GET /ipfs/QmVRzPKPzNtSrEzBFm2UZfxmPAgnaLke4DMcerbsGGSaFe/link)
type gatewayHandler struct {
	node   *core.IpfsNode
	config GatewayConfig
	api    coreiface.CoreAPI
}

func newGatewayHandler(n *core.IpfsNode, c GatewayConfig, api coreiface.CoreAPI) *gatewayHandler {
	i := &gatewayHandler{
		node:   n,
		config: c,
		api:    api,
	}
	return i
}

// TODO(cryptix):  find these helpers somewhere else
func (i *gatewayHandler) newDagFromReader(r io.Reader) (node.Node, error) {
	// TODO(cryptix): change and remove this helper once PR1136 is merged
	// return ufs.AddFromReader(i.node, r.Body)
	return importer.BuildDagFromReader(
		i.node.DAG,
		chunk.DefaultSplitter(r))
}

// TODO(btc): break this apart into separate handlers using a more expressive muxer
func (i *gatewayHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(i.node.Context(), time.Hour)
	// the hour is a hard fallback, we don't expect it to happen, but just in case
	defer cancel()

	if cn, ok := w.(http.CloseNotifier); ok {
		clientGone := cn.CloseNotify()
		go func() {
			select {
			case <-clientGone:
			case <-ctx.Done():
			}
			cancel()
		}()
	}

	defer func() {
		if r := recover(); r != nil {
			log.Error("A panic occurred in the gateway handler!")
			log.Error(r)
			debug.PrintStack()
		}
	}()

	if len(i.config.AllowedIPs) > 0 {
		remoteAddr := strings.Split(r.RemoteAddr, ":")
		if !i.config.AllowedIPs[remoteAddr[0]] {
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprint(w, "403 - Forbidden")
			return
		}
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

	if i.config.Writable {
		switch r.Method {
		case "POST":
			i.postHandler(ctx, w, r)
			return
		case "PUT":
			i.putHandler(w, r)
			return
		case "DELETE":
			i.deleteHandler(w, r)
			return
		}
	}

	if r.Method == "GET" || r.Method == "HEAD" {
		i.getOrHeadHandler(ctx, w, r)
		return
	}

	if r.Method == "OPTIONS" {
		i.optionsHandler(w, r)
		return
	}

	errmsg := "Method " + r.Method + " not allowed: "
	if !i.config.Writable {
		w.WriteHeader(http.StatusMethodNotAllowed)
		errmsg = errmsg + "read only access"
	} else {
		w.WriteHeader(http.StatusBadRequest)
		errmsg = errmsg + "bad request for " + r.URL.Path
	}
	fmt.Fprint(w, errmsg)
	log.Error(errmsg) // TODO(cryptix): log errors until we have a better way to expose these (counter metrics maybe)
}

func (i *gatewayHandler) optionsHandler(w http.ResponseWriter, r *http.Request) {
	/*
		OPTIONS is a noop request that is used by the browsers to check
		if server accepts cross-site XMLHttpRequest (indicated by the presence of CORS headers)
		https://developer.mozilla.org/en-US/docs/Web/HTTP/Access_control_CORS#Preflighted_requests
	*/
	i.addUserHeaders(w) // return all custom headers (including CORS ones, if set)
}

func (i *gatewayHandler) getOrHeadHandler(ctx context.Context, w http.ResponseWriter, r *http.Request) {

	// If this is an ipns query and the user passed in a blockchain ID handle, let's resolve it into a peer ID.
	var paths []string = strings.Split(r.URL.Path, "/")
	if paths[1] == "ipns" && paths[2][0:1] == "@" {
		peerID, err := i.config.Resolver.Resolve(paths[2])
		if err != nil {
			webError(w, "Path Resolve error", err, http.StatusBadRequest)
			return
		}
		r.URL.Path = strings.Replace(r.URL.Path, paths[2], peerID, 1)
	}

	if paths[1] == "ipfs" && paths[2] == "providers" {
		dht, ok := i.node.Routing.(*dht.IpfsDHT)
		if !ok {
			webError(w, "Dht routing not available", nil, http.StatusInternalServerError)
			return
		}
		if len(paths) < 3 || paths[3] == "" {
			webError(w, "Must specify key in path", nil, http.StatusBadRequest)
			return
		}
		key, err := cid.Decode(paths[3])
		if err != nil {
			webError(w, "Invalid key", nil, http.StatusBadRequest)
			return
		}
		pmes := dhtpb.NewMessage(dhtpb.Message_GET_PROVIDERS, key.KeyString(), 0)
		provs, err := dht.GetProviders(ctx, pmes)
		if err != nil {
			webError(w, "Error looking up providers", err, http.StatusInternalServerError)
			return
		}
		resp, err := proto.Marshal(provs)
		if err != nil {
			webError(w, "Error marshalling response", err, http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Disposition", "attachment; filename=providers.pb")
		w.Header().Set("Content-Type", r.Header.Get("Content-Type"))

		w.Write(resp)
		return
	}

	unmodifiedURLPath := r.URL.Path

	// If this is an ipns query let's check to see if it's using our own peer ID.
	// If so let's resolve it locally instead of going out to the network.
	var ownID bool = false
	if paths[1] == "ipns" && paths[2] == i.node.Identity.Pretty() {
		id := i.node.Identity
		_, ipnskey := namesys.IpnsKeysForID(id)
		ival, hasherr := i.node.Repo.Datastore().Get(dshelp.NewKeyFromBinary([]byte(ipnskey)))
		if hasherr != nil {
			webError(w, "Error fetching own IPNS mapping", hasherr, http.StatusInternalServerError)
			return
		}
		val := ival.([]byte)
		dhtrec := new(recpb.Record)
		proto.Unmarshal(val, dhtrec)
		e := new(namepb.IpnsEntry)
		proto.Unmarshal(dhtrec.GetValue(), e)
		pth := path.Path(e.Value).String()
		for _, p := range paths[3:] {
			pth += "/" + p
		}
		r.URL.Path = pth
		ownID = true
	}

	urlPath := r.URL.Path

	// If the gateway is behind a reverse proxy and mounted at a sub-path,
	// the prefix header can be set to signal this sub-path.
	// It will be prepended to links in directory listings and the index.html redirect.
	prefix := ""
	if prefixHdr := r.Header["X-Ipfs-Gateway-Prefix"]; len(prefixHdr) > 0 {
		prfx := prefixHdr[0]
		for _, p := range i.config.PathPrefixes {
			if prfx == p || strings.HasPrefix(prfx, p+"/") {
				prefix = prfx
				break
			}
		}
	}

	// IPNSHostnameOption might have constructed an IPNS path using the Host header.
	// In this case, we need the original path for constructing redirects
	// and links that match the requested URL.
	// For example, http://example.net would become /ipns/example.net, and
	// the redirects and links would end up as http://example.net/ipns/example.net
	originalUrlPath := prefix + urlPath
	ipnsHostname := false
	if hdr := r.Header["X-Ipns-Original-Path"]; len(hdr) > 0 {
		originalUrlPath = prefix + hdr[0]
		ipnsHostname = true
	}
	parsedPath, err := coreapi.ParsePath(urlPath)
	if err != nil {
		webError(w, "invalid ipfs path", err, http.StatusBadRequest)
		return
	}

	// Resolve path to the final DAG node for the ETag
	resolvedPath, err := i.api.ResolvePath(ctx, parsedPath)
	switch err {
	case nil:
	case coreiface.ErrOffline:
		if !i.node.OnlineMode() {
			webError(w, "ipfs resolve -r "+urlPath, err, http.StatusServiceUnavailable)
			return
		}
		fallthrough
	default:
		webError(w, "ipfs resolve -r "+urlPath, err, http.StatusNotFound)
		return
	}

	dr, err := i.api.Unixfs().Cat(ctx, resolvedPath)
	dir := false
	switch err {
	case nil:
		// Cat() worked
		defer dr.Close()
	case coreiface.ErrIsDir:
		dir = true
	default:
		webError(w, "ipfs cat "+urlPath, err, http.StatusNotFound)
		return
	}

	// Check etag send back to us
	etag := "\"" + resolvedPath.Cid().String() + "\""
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	i.addUserHeaders(w) // ok, _now_ write user's headers.
	w.Header().Set("X-IPFS-Path", urlPath)
	w.Header().Set("Etag", etag)

	// set 'allowed' headers
	w.Header().Set("Access-Control-Allow-Headers", "X-Stream-Output, X-Chunked-Output")
	// expose those headers
	w.Header().Set("Access-Control-Expose-Headers", "X-Stream-Output, X-Chunked-Output")

	// Suborigin header, sandboxes apps from each other in the browser (even
	// though they are served from the same gateway domain).
	//
	// Omitted if the path was treated by IPNSHostnameOption(), for example
	// a request for http://example.net/ would be changed to /ipns/example.net/,
	// which would turn into an incorrect Suborigin header.
	// In this case the correct thing to do is omit the header because it is already
	// handled correctly without a Suborigin.
	//
	// NOTE: This is not yet widely supported by browsers.
	if !ipnsHostname {
		// e.g.: 1="ipfs", 2="QmYuNaKwY...", ...
		pathComponents := strings.SplitN(urlPath, "/", 4)

		var suboriginRaw []byte
		cidDecoded, err := cid.Decode(pathComponents[2])
		if err != nil {
			// component 2 doesn't decode with cid, so it must be a hostname
			suboriginRaw = []byte(strings.ToLower(pathComponents[2]))
		} else {
			suboriginRaw = cidDecoded.Bytes()
		}

		base32Encoded, err := multibase.Encode(multibase.Base32, suboriginRaw)
		if err != nil {
			internalWebError(w, err)
			return
		}

		suborigin := pathComponents[1] + "000" + strings.ToLower(base32Encoded)
		w.Header().Set("Suborigin", suborigin)
	}

	// set these headers _after_ the error, for we may just not have it
	// and dont want the client to cache a 500 response...
	// and only if it's /ipfs!
	// TODO: break this out when we split /ipfs /ipns routes.
	modtime := time.Now()
	if strings.HasPrefix(urlPath, ipfsPathPrefix) && !dir {
		w.Header().Set("Cache-Control", "public, max-age=29030400, immutable")
		// set modtime to a really long time ago, since files are immutable and should stay cached
		modtime = time.Unix(1, 0)
	} else if strings.HasPrefix(unmodifiedURLPath, ipnsPathPrefix) && !ownID { // cache ipns returns for 10 minutes
		w.Header().Set("Cache-Control", "public, max-age=600, immutable")
	}

	if !dir {
		name := gopath.Base(urlPath)
		http.ServeContent(w, r, name, modtime, dr)
		return
	}

	nd, err := i.api.ResolveNode(ctx, resolvedPath)
	if err != nil {
		internalWebError(w, err)
		return
	}

	dirr, err := uio.NewDirectoryFromNode(i.node.DAG, nd)
	if err != nil {
		internalWebError(w, err)
		return
	}

	ixnd, err := dirr.Find(ctx, "index.html")
	switch {
	case err == nil:
		log.Debugf("found index.html link for %s", urlPath)

		dirwithoutslash := urlPath[len(urlPath)-1] != '/'
		goget := r.URL.Query().Get("go-get") == "1"
		if dirwithoutslash && !goget {
			// See comment above where originalUrlPath is declared.
			http.Redirect(w, r, originalUrlPath+"/", 302)
			log.Debugf("redirect to %s", originalUrlPath+"/")
			return
		}

		dr, err := i.api.Unixfs().Cat(ctx, coreapi.ParseCid(ixnd.Cid()))
		if err != nil {
			internalWebError(w, err)
			return
		}
		defer dr.Close()

		// write to request
		http.ServeContent(w, r, "index.html", modtime, dr)
		return
	default:
		internalWebError(w, err)
		return
	case os.IsNotExist(err):
	}

	if r.Method == "HEAD" {
		return
	}

	// storage for directory listing
	var dirListing []directoryItem
	dirr.ForEachLink(ctx, func(link *node.Link) error {
		// See comment above where originalUrlPath is declared.
		di := directoryItem{humanize.Bytes(link.Size), link.Name, gopath.Join(originalUrlPath, link.Name)}
		dirListing = append(dirListing, di)
		return nil
	})

	// construct the correct back link
	// https://github.com/ipfs/go-ipfs/issues/1365
	var backLink string = prefix + urlPath

	// don't go further up than /ipfs/$hash/
	pathSplit := path.SplitList(backLink)
	switch {
	// keep backlink
	case len(pathSplit) == 3: // url: /ipfs/$hash

	// keep backlink
	case len(pathSplit) == 4 && pathSplit[3] == "": // url: /ipfs/$hash/

	// add the correct link depending on wether the path ends with a slash
	default:
		if strings.HasSuffix(backLink, "/") {
			backLink += "./.."
		} else {
			backLink += "/.."
		}
	}

	// strip /ipfs/$hash from backlink if IPNSHostnameOption touched the path.
	if ipnsHostname {
		backLink = prefix + "/"
		if len(pathSplit) > 5 {
			// also strip the trailing segment, because it's a backlink
			backLinkParts := pathSplit[3 : len(pathSplit)-2]
			backLink += path.Join(backLinkParts) + "/"
		}
	}

	// See comment above where originalUrlPath is declared.
	tplData := listingTemplateData{
		Listing:  dirListing,
		Path:     originalUrlPath,
		BackLink: backLink,
	}
	err = listingTemplate.Execute(w, tplData)
	if err != nil {
		internalWebError(w, err)
		return
	}
}

func (i *gatewayHandler) postHandler(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	var paths []string = strings.Split(r.URL.Path, "/")
	if len(paths) > 1 && paths[1] == "ipfs" && paths[2] == "providers" {
		// handle provider post
		dht, ok := i.node.Routing.(*dht.IpfsDHT)
		if !ok {
			webError(w, "Dht routing not available", nil, http.StatusInternalServerError)
			return
		}
		buf, err := ioutil.ReadAll(r.Body)
		if err != nil {
			webError(w, "Invalid provider data", err, http.StatusBadRequest)
			return
		}
		pmes := new(dhtpb.Message)
		err = proto.Unmarshal(buf, pmes)
		if err != nil {
			webError(w, "Error unmarshalling message", err, http.StatusBadRequest)
			return
		}
		err = dht.AddProvider(ctx, pmes)
		if err != nil {
			webError(w, "Error adding provier", err, http.StatusInternalServerError)
		}
		return
	}

	p, err := i.api.Unixfs().Add(ctx, r.Body)
	if err != nil {
		internalWebError(w, err)
		return
	}

	i.addUserHeaders(w) // ok, _now_ write user's headers.
	w.Header().Set("IPFS-Hash", p.Cid().String())
	http.Redirect(w, r, p.String(), http.StatusCreated)
}

func (i *gatewayHandler) putHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(i.node.Context(), time.Second*300)

	var paths []string = strings.Split(r.URL.Path, "/")
	if paths[1] != "ipfs" {
		webError(w, "Cannot put to IPNS", errors.New("Cannot put to IPNS"), http.StatusInternalServerError)
		cancel()
		return
	}
	if len(paths) != 3 {
		webError(w, "Path must contain only one hash", errors.New("Path must contain only one hash"), http.StatusInternalServerError)
		cancel()
		return
	}

	go func() {
		k, err := cid.Decode(paths[2])
		if err != nil {
			return
		}
		dag.FetchGraph(ctx, k, i.node.DAG)
	}()

	i.addUserHeaders(w)
	return
}

func (i *gatewayHandler) deleteHandler(w http.ResponseWriter, r *http.Request) {
	urlPath := r.URL.Path
	ctx, cancel := context.WithCancel(i.node.Context())
	defer cancel()

	p, err := path.ParsePath(urlPath)
	if err != nil {
		webError(w, "failed to parse path", err, http.StatusBadRequest)
		return
	}

	c, components, err := path.SplitAbsPath(p)
	if err != nil {
		webError(w, "Could not split path", err, http.StatusInternalServerError)
		return
	}

	tctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	rootnd, err := i.node.Resolver.DAG.Get(tctx, c)
	if err != nil {
		webError(w, "Could not resolve root object", err, http.StatusBadRequest)
		return
	}

	pathNodes, err := i.node.Resolver.ResolveLinks(tctx, rootnd, components[:len(components)-1])
	if err != nil {
		webError(w, "Could not resolve parent object", err, http.StatusBadRequest)
		return
	}

	pbnd, ok := pathNodes[len(pathNodes)-1].(*dag.ProtoNode)
	if !ok {
		webError(w, "Cannot read non protobuf nodes through gateway", dag.ErrNotProtobuf, http.StatusBadRequest)
		return
	}

	// TODO(cyrptix): assumes len(pathNodes) > 1 - not found is an error above?
	err = pbnd.RemoveNodeLink(components[len(components)-1])
	if err != nil {
		webError(w, "Could not delete link", err, http.StatusBadRequest)
		return
	}

	var newnode *dag.ProtoNode = pbnd
	for j := len(pathNodes) - 2; j >= 0; j-- {
		if _, err := i.node.DAG.Add(newnode); err != nil {
			webError(w, "Could not add node", err, http.StatusInternalServerError)
			return
		}

		pathpb, ok := pathNodes[j].(*dag.ProtoNode)
		if !ok {
			webError(w, "Cannot read non protobuf nodes through gateway", dag.ErrNotProtobuf, http.StatusBadRequest)
			return
		}

		newnode, err = pathpb.UpdateNodeLink(components[j], newnode)
		if err != nil {
			webError(w, "Could not update node links", err, http.StatusInternalServerError)
			return
		}
	}

	if _, err := i.node.DAG.Add(newnode); err != nil {
		webError(w, "Could not add root node", err, http.StatusInternalServerError)
		return
	}

	// Redirect to new path
	ncid := newnode.Cid()

	i.addUserHeaders(w) // ok, _now_ write user's headers.
	w.Header().Set("IPFS-Hash", ncid.String())
	http.Redirect(w, r, gopath.Join(ipfsPathPrefix+ncid.String(), path.Join(components[:len(components)-1])), http.StatusCreated)
}

func (i *gatewayHandler) addUserHeaders(w http.ResponseWriter) {
	for k, v := range i.config.Headers {
		w.Header()[k] = v
	}
}

func webError(w http.ResponseWriter, message string, err error, defaultCode int) {
	if _, ok := err.(path.ErrNoLink); ok {
		webErrorWithCode(w, message, err, http.StatusNotFound)
	} else if err == routing.ErrNotFound {
		webErrorWithCode(w, message, err, http.StatusNotFound)
	} else if err == context.DeadlineExceeded {
		webErrorWithCode(w, message, err, http.StatusRequestTimeout)
	} else {
		webErrorWithCode(w, message, err, defaultCode)
	}
}

func webErrorWithCode(w http.ResponseWriter, message string, err error, code int) {
	w.WriteHeader(code)

	log.Errorf("%s: %s", message, err) // TODO(cryptix): log until we have a better way to expose these (counter metrics maybe)
	fmt.Fprintf(w, "%s: %s\n", message, err)
}

// return a 500 error and log
func internalWebError(w http.ResponseWriter, err error) {
	webErrorWithCode(w, "internalWebError", err, http.StatusInternalServerError)
}
