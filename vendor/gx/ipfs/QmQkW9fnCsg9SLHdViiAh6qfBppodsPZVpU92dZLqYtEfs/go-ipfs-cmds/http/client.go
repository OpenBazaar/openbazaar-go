package http

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"

	cmds "gx/ipfs/QmQkW9fnCsg9SLHdViiAh6qfBppodsPZVpU92dZLqYtEfs/go-ipfs-cmds"

	"gx/ipfs/QmQmhotPUzVrMEWNK3x1R5jQ5ZHWyL7tVUrmRPjrBrvyCb/go-ipfs-files"
	"gx/ipfs/Qmde5VP1qUkyQXKCfmEUA7bP64V2HAptbJ7phuPp7jXWwg/go-ipfs-cmdkit"
)

const (
	ApiUrlFormat = "%s%s/%s?%s"
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

	if cmd.PreRun != nil {
		err := cmd.PreRun(req, env)
		if err != nil {
			return err
		}
	}

	res, err := c.Send(req)
	if err != nil {
		if isConnRefused(err) {
			err = fmt.Errorf("cannot connect to the api. Is the deamon running? To run as a standalone CLI command remove the api file in `$IPFS_PATH/api`")
		}
		return err
	}

	if cmd.PostRun != nil {
		if typer, ok := re.(interface {
			Type() cmds.PostRunType
		}); ok && cmd.PostRun[typer.Type()] != nil {
			err := cmd.PostRun[typer.Type()](res, re)
			closeErr := re.CloseWithError(err)
			if closeErr == cmds.ErrClosingClosedEmitter {
				// ignore double close errors
				return nil
			}

			return err
		}
	}

	return cmds.Copy(re, res)
}

func (c *client) toHTTPRequest(req *cmds.Request) (*http.Request, error) {
	query, err := getQuery(req)
	if err != nil {
		return nil, err
	}

	var fileReader *files.MultiFileReader
	var reader io.Reader // in case we have no body to send we need to provide
	// untyped nil to http.NewRequest

	if bodyArgs := req.BodyArgs(); bodyArgs != nil {
		// In the end, this wraps a file reader in a file reader.
		// However, such is life.
		fileReader = files.NewMultiFileReader(files.NewMapDirectory(map[string]files.Node{
			"stdin": files.NewReaderFile(bodyArgs),
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

	return httpReq, nil
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

	// build http request
	httpReq, err := c.toHTTPRequest(req)
	if err != nil {
		return nil, err
	}

	// send http request
	httpRes, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}

	// parse using the overridden JSON encoding in request
	res, err := parseResponse(httpRes, req)
	if err != nil {
		return nil, err
	}

	// reset request encoding to what it was before
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
