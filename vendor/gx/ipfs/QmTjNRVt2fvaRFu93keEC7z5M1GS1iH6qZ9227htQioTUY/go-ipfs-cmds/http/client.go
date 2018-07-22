package http

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"

	cmds "gx/ipfs/QmTjNRVt2fvaRFu93keEC7z5M1GS1iH6qZ9227htQioTUY/go-ipfs-cmds"
	"gx/ipfs/QmceUdzxkimdYsgtX733uNgzf1DLHyBKN6ehGSp85ayppM/go-ipfs-cmdkit"
	"gx/ipfs/QmceUdzxkimdYsgtX733uNgzf1DLHyBKN6ehGSp85ayppM/go-ipfs-cmdkit/files"
)

const (
	ApiUrlFormat = "%s%s/%s?%s"
)

var (
	ErrAPINotRunning = errors.New("api not running")
)

var OptionSkipMap = map[string]bool{
	"api": true,
}

// Client is the commands HTTP client interface.
type Client interface {
	Send(req *cmds.Request) (cmds.Response, error)
}

type client struct {
	serverAddress string
	httpClient    *http.Client
	ua            string
	apiPrefix     string
}

type ClientOpt func(*client)

func ClientWithUserAgent(ua string) ClientOpt {
	return func(c *client) {
		c.ua = ua
	}
}

func ClientWithAPIPrefix(apiPrefix string) ClientOpt {
	return func(c *client) {
		c.apiPrefix = apiPrefix
	}
}

func NewClient(address string, opts ...ClientOpt) Client {
	if !strings.HasPrefix(address, "http://") {
		address = "http://" + address
	}

	c := &client{
		serverAddress: address,
		httpClient:    http.DefaultClient,
		ua:            "go-ipfs-cmds/http",
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

func (c *client) Execute(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
	cmd := req.Command

	// If this ResponseEmitter encodes messages (e.g. http, cli or writer - but not chan),
	// we need to update the encoding to the one specified by the command.
	if ee, ok := re.(cmds.EncodingEmitter); ok {
		encType := cmds.GetEncoding(req)

		// note the difference: cmd.Encoders vs. cmds.Encoders
		if enc, ok := cmd.Encoders[encType]; ok {
			ee.SetEncoder(enc(req))
		} else if enc, ok := cmds.Encoders[encType]; ok {
			ee.SetEncoder(enc(req))
		} else {
			log.Errorf("unknown encoding %q, using json", encType)
			ee.SetEncoder(cmds.Encoders[cmds.JSON](req))
		}
	}

	if cmd.PreRun != nil {
		err := cmd.PreRun(req, env)
		if err != nil {
			return err
		}
	}

	if cmd.PostRun != nil {
		if typer, ok := re.(interface {
			Type() cmds.PostRunType
		}); ok && cmd.PostRun[typer.Type()] != nil {
			re = cmd.PostRun[typer.Type()](req, re)
		}
	}

	res, err := c.Send(req)
	if err != nil {
		if isConnRefused(err) {
			err = ErrAPINotRunning
		}
		return err
	}

	return cmds.Copy(re, res)
}

func (c *client) Send(req *cmds.Request) (cmds.Response, error) {
	if req.Context == nil {
		log.Warningf("no context set in request")
		req.Context = context.Background()
	}

	// save user-provided encoding
	previousUserProvidedEncoding, found := req.Options[cmds.EncLong].(string)

	// override with json to send to server
	req.SetOption(cmds.EncLong, cmds.JSON)

	// stream channel output
	req.SetOption(cmds.ChanOpt, true)

	query, err := getQuery(req)
	if err != nil {
		return nil, err
	}

	var fileReader *files.MultiFileReader
	var reader io.Reader
	if bodyArgs := req.BodyArgs(); bodyArgs != nil {
		// In the end, this wraps a file reader in a file reader.
		// However, such is life.
		fileReader = files.NewMultiFileReader(files.NewSliceFile("", "", []files.File{
			files.NewReaderFile("stdin", "", bodyArgs, nil),
		}), true)
		reader = fileReader
	} else if req.Files != nil {
		fileReader = files.NewMultiFileReader(req.Files, true)
		reader = fileReader
	}

	path := strings.Join(req.Path, "/")
	url := fmt.Sprintf(ApiUrlFormat, c.serverAddress, c.apiPrefix, path, query)

	httpReq, err := http.NewRequest("POST", url, reader)
	if err != nil {
		return nil, err
	}

	// TODO extract string consts?
	if fileReader != nil {
		httpReq.Header.Set(contentTypeHeader, "multipart/form-data; boundary="+fileReader.Boundary())
	} else {
		httpReq.Header.Set(contentTypeHeader, applicationOctetStream)
	}
	httpReq.Header.Set(uaHeader, c.ua)

	httpReq = httpReq.WithContext(req.Context)
	httpReq.Close = true

	httpRes, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}

	// using the overridden JSON encoding in request
	res, err := parseResponse(httpRes, req)
	if err != nil {
		return nil, err
	}

	if found && len(previousUserProvidedEncoding) > 0 {
		// reset to user provided encoding after sending request
		// NB: if user has provided an encoding but it is the empty string,
		// still leave it as JSON.
		req.SetOption(cmds.EncLong, previousUserProvidedEncoding)
	}

	return res, nil
}

func getQuery(req *cmds.Request) (string, error) {
	query := url.Values{}

	for k, v := range req.Options {
		if OptionSkipMap[k] {
			continue
		}
		str := fmt.Sprintf("%v", v)
		query.Set(k, str)
	}

	args := req.Arguments
	argDefs := req.Command.Arguments

	argDefIndex := 0

	for _, arg := range args {
		argDef := argDefs[argDefIndex]
		// skip ArgFiles
		for argDef.Type == cmdkit.ArgFile {
			argDefIndex++
			argDef = argDefs[argDefIndex]
		}

		query.Add("arg", arg)

		if len(argDefs) > argDefIndex+1 {
			argDefIndex++
		}
	}

	return query.Encode(), nil
}

func isConnRefused(err error) bool {
	// unwrap url errors from http calls
	if urlerr, ok := err.(*url.Error); ok {
		err = urlerr.Err
	}

	netoperr, ok := err.(*net.OpError)
	if !ok {
		return false
	}

	return netoperr.Op == "dial"
}
