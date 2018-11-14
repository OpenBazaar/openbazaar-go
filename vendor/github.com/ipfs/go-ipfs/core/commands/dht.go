package commands

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	cmds "github.com/ipfs/go-ipfs/commands"
	e "github.com/ipfs/go-ipfs/core/commands/e"
	dag "gx/ipfs/QmSei8kFMfqdJq7Q68d2LMnHbTWKKg2daA29ezUYFAUNgc/go-merkledag"
	path "gx/ipfs/QmT3rzed1ppXefourpmoZ7tyVQfsGPQZ1pHDngLmCvXxd3/go-path"

	cid "gx/ipfs/QmPSQnBKM9g7BaUcZCvswUJVscQ1ipjmwxN5PXCjkp9EQ7/go-cid"
	ipld "gx/ipfs/QmR7TcHkR9nxkUorfi8XMTAMLUK7GiP64TWWBzY3aacc1o/go-ipld-format"
	peer "gx/ipfs/QmTRhk7cgjUf2gfQ3p2M9KPECNZEW9XUrmHcFCgog4cPgB/go-libp2p-peer"
	pstore "gx/ipfs/QmTTJcDL3gsnGDALjh2fDGg1onGRUdVgNL2hU2WEZcVrMX/go-libp2p-peerstore"
	b58 "gx/ipfs/QmWFAMPqsEyUX7gDUsRVmMWz59FxSpJ1b2v6bJ1yYzo7jY/go-base58-fast/base58"
	routing "gx/ipfs/QmcQ81jSyWCp1jpkQ8CMbtpXT3jK7Wg6ZtYmoyWFgBoF9c/go-libp2p-routing"
	notif "gx/ipfs/QmcQ81jSyWCp1jpkQ8CMbtpXT3jK7Wg6ZtYmoyWFgBoF9c/go-libp2p-routing/notifications"
	"gx/ipfs/Qmde5VP1qUkyQXKCfmEUA7bP64V2HAptbJ7phuPp7jXWwg/go-ipfs-cmdkit"
)

var ErrNotDHT = errors.New("routing service is not a DHT")

// TODO: Factor into `ipfs dht` and `ipfs routing`.
// Everything *except `query` goes into `ipfs routing`.

var DhtCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline:          "Issue commands directly through the DHT.",
		ShortDescription: ``,
	},

	Subcommands: map[string]*cmds.Command{
		"query":     queryDhtCmd,
		"findprovs": findProvidersDhtCmd,
		"findpeer":  findPeerDhtCmd,
		"get":       getValueDhtCmd,
		"put":       putValueDhtCmd,
		"provide":   provideRefDhtCmd,
	},
}

const (
	dhtVerboseOptionName = "v"
)

var queryDhtCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline:          "Find the closest Peer IDs to a given Peer ID by querying the DHT.",
		ShortDescription: "Outputs a list of newline-delimited Peer IDs.",
	},

	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("peerID", true, true, "The peerID to run the query against."),
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption("verbose", dhtVerboseOptionName, "Print extra information."),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		if n.DHT == nil {
			res.SetError(ErrNotDHT, cmdkit.ErrNormal)
			return
		}

		id, err := peer.IDB58Decode(req.Arguments()[0])
		if err != nil {
			res.SetError(cmds.ClientError("invalid peer ID"), cmdkit.ErrClient)
			return
		}

		ctx, cancel := context.WithCancel(req.Context())
		ctx, events := notif.RegisterForQueryEvents(ctx)

		closestPeers, err := n.DHT.GetClosestPeers(ctx, string(id))
		if err != nil {
			cancel()
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		go func() {
			defer cancel()
			for p := range closestPeers {
				notif.PublishQueryEvent(ctx, &notif.QueryEvent{
					ID:   p,
					Type: notif.FinalPeer,
				})
			}
		}()

		outChan := make(chan interface{})
		res.SetOutput((<-chan interface{})(outChan))

		go func() {
			defer close(outChan)
			for e := range events {
				select {
				case outChan <- e:
				case <-req.Context().Done():
					return
				}
			}
		}()
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func() cmds.Marshaler {
			pfm := pfuncMap{
				notif.PeerResponse: func(obj *notif.QueryEvent, out io.Writer, verbose bool) {
					for _, p := range obj.Responses {
						fmt.Fprintf(out, "%s\n", p.ID.Pretty())
					}
				},
			}

			return func(res cmds.Response) (io.Reader, error) {
				v, err := unwrapOutput(res.Output())
				if err != nil {
					return nil, err
				}

				obj, ok := v.(*notif.QueryEvent)
				if !ok {
					return nil, e.TypeErr(obj, v)
				}

				verbose, _, _ := res.Request().Option(dhtVerboseOptionName).Bool()

				buf := new(bytes.Buffer)
				printEvent(obj, buf, verbose, pfm)
				return buf, nil
			}
		}(),
	},
	Type: notif.QueryEvent{},
}

const (
	numProvidersOptionName = "num-providers"
)

var findProvidersDhtCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline:          "Find peers that can provide a specific value, given a key.",
		ShortDescription: "Outputs a list of newline-delimited provider Peer IDs.",
	},

	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("key", true, true, "The key to find providers for."),
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption("verbose", dhtVerboseOptionName, "Print extra information."),
		cmdkit.IntOption(numProvidersOptionName, "n", "The number of providers to find.").WithDefault(20),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		if n.Routing == nil {
			res.SetError(ErrNotOnline, cmdkit.ErrNormal)
			return
		}

		numProviders, _, err := res.Request().Option(numProvidersOptionName).Int()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}
		if numProviders < 1 {
			res.SetError(fmt.Errorf("number of providers must be greater than 0"), cmdkit.ErrNormal)
			return
		}

		c, err := cid.Parse(req.Arguments()[0])

		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		outChan := make(chan interface{})
		res.SetOutput((<-chan interface{})(outChan))

		ctx, cancel := context.WithCancel(req.Context())
		ctx, events := notif.RegisterForQueryEvents(ctx)

		pchan := n.Routing.FindProvidersAsync(ctx, c, numProviders)
		go func() {
			defer close(outChan)
			for e := range events {
				select {
				case outChan <- e:
				case <-req.Context().Done():
					return
				}
			}
		}()

		go func() {
			defer cancel()
			for p := range pchan {
				np := p
				notif.PublishQueryEvent(ctx, &notif.QueryEvent{
					Type:      notif.Provider,
					Responses: []*pstore.PeerInfo{&np},
				})
			}
		}()
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func() func(cmds.Response) (io.Reader, error) {
			pfm := pfuncMap{
				notif.FinalPeer: func(obj *notif.QueryEvent, out io.Writer, verbose bool) {
					if verbose {
						fmt.Fprintf(out, "* closest peer %s\n", obj.ID)
					}
				},
				notif.Provider: func(obj *notif.QueryEvent, out io.Writer, verbose bool) {
					prov := obj.Responses[0]
					if verbose {
						fmt.Fprintf(out, "provider: ")
					}
					fmt.Fprintf(out, "%s\n", prov.ID.Pretty())
					if verbose {
						for _, a := range prov.Addrs {
							fmt.Fprintf(out, "\t%s\n", a)
						}
					}
				},
			}

			return func(res cmds.Response) (io.Reader, error) {
				verbose, _, _ := res.Request().Option(dhtVerboseOptionName).Bool()
				v, err := unwrapOutput(res.Output())
				if err != nil {
					return nil, err
				}

				obj, ok := v.(*notif.QueryEvent)
				if !ok {
					return nil, e.TypeErr(obj, v)
				}

				buf := new(bytes.Buffer)
				printEvent(obj, buf, verbose, pfm)
				return buf, nil
			}
		}(),
	},
	Type: notif.QueryEvent{},
}

const (
	recursiveOptionName = "recursive"
)

var provideRefDhtCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Announce to the network that you are providing given values.",
	},

	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("key", true, true, "The key[s] to send provide records for.").EnableStdin(),
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption("verbose", dhtVerboseOptionName, "Print extra information."),
		cmdkit.BoolOption(recursiveOptionName, "r", "Recursively provide entire graph."),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		if n.Routing == nil {
			res.SetError(ErrNotOnline, cmdkit.ErrNormal)
			return
		}

		if len(n.PeerHost.Network().Conns()) == 0 {
			res.SetError(errors.New("cannot provide, no connected peers"), cmdkit.ErrNormal)
			return
		}

		rec, _, _ := req.Option(recursiveOptionName).Bool()

		var cids []cid.Cid
		for _, arg := range req.Arguments() {
			c, err := cid.Decode(arg)
			if err != nil {
				res.SetError(err, cmdkit.ErrNormal)
				return
			}

			has, err := n.Blockstore.Has(c)
			if err != nil {
				res.SetError(err, cmdkit.ErrNormal)
				return
			}

			if !has {
				res.SetError(fmt.Errorf("block %s not found locally, cannot provide", c), cmdkit.ErrNormal)
				return
			}

			cids = append(cids, c)
		}

		outChan := make(chan interface{})
		res.SetOutput((<-chan interface{})(outChan))

		ctx, cancel := context.WithCancel(req.Context())
		ctx, events := notif.RegisterForQueryEvents(ctx)

		go func() {
			defer close(outChan)
			for e := range events {
				select {
				case outChan <- e:
				case <-req.Context().Done():
					return
				}
			}
		}()

		go func() {
			defer cancel()
			var err error
			if rec {
				err = provideKeysRec(ctx, n.Routing, n.DAG, cids)
			} else {
				err = provideKeys(ctx, n.Routing, cids)
			}
			if err != nil {
				notif.PublishQueryEvent(ctx, &notif.QueryEvent{
					Type:  notif.QueryError,
					Extra: err.Error(),
				})
			}
		}()
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func() func(res cmds.Response) (io.Reader, error) {
			pfm := pfuncMap{
				notif.FinalPeer: func(obj *notif.QueryEvent, out io.Writer, verbose bool) {
					if verbose {
						fmt.Fprintf(out, "sending provider record to peer %s\n", obj.ID)
					}
				},
			}

			return func(res cmds.Response) (io.Reader, error) {
				verbose, _, _ := res.Request().Option(dhtVerboseOptionName).Bool()
				v, err := unwrapOutput(res.Output())
				if err != nil {
					return nil, err
				}
				obj, ok := v.(*notif.QueryEvent)
				if !ok {
					return nil, e.TypeErr(obj, v)
				}

				buf := new(bytes.Buffer)
				printEvent(obj, buf, verbose, pfm)
				return buf, nil
			}
		}(),
	},
	Type: notif.QueryEvent{},
}

func provideKeys(ctx context.Context, r routing.IpfsRouting, cids []cid.Cid) error {
	for _, c := range cids {
		err := r.Provide(ctx, c, true)
		if err != nil {
			return err
		}
	}
	return nil
}

func provideKeysRec(ctx context.Context, r routing.IpfsRouting, dserv ipld.DAGService, cids []cid.Cid) error {
	provided := cid.NewSet()
	for _, c := range cids {
		kset := cid.NewSet()

		err := dag.EnumerateChildrenAsync(ctx, dag.GetLinksDirect(dserv), c, kset.Visit)
		if err != nil {
			return err
		}

		for _, k := range kset.Keys() {
			if provided.Has(k) {
				continue
			}

			err = r.Provide(ctx, k, true)
			if err != nil {
				return err
			}
			provided.Add(k)
		}
	}

	return nil
}

var findPeerDhtCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline:          "Find the multiaddresses associated with a Peer ID.",
		ShortDescription: "Outputs a list of newline-delimited multiaddresses.",
	},

	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("peerID", true, true, "The ID of the peer to search for."),
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption("verbose", dhtVerboseOptionName, "Print extra information."),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		if n.Routing == nil {
			res.SetError(ErrNotOnline, cmdkit.ErrNormal)
			return
		}

		pid, err := peer.IDB58Decode(req.Arguments()[0])
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		outChan := make(chan interface{})
		res.SetOutput((<-chan interface{})(outChan))

		ctx, cancel := context.WithCancel(req.Context())
		ctx, events := notif.RegisterForQueryEvents(ctx)

		go func() {
			defer close(outChan)
			for v := range events {
				select {
				case outChan <- v:
				case <-req.Context().Done():
				}

			}
		}()

		go func() {
			defer cancel()
			pi, err := n.Routing.FindPeer(ctx, pid)
			if err != nil {
				notif.PublishQueryEvent(ctx, &notif.QueryEvent{
					Type:  notif.QueryError,
					Extra: err.Error(),
				})
				return
			}

			notif.PublishQueryEvent(ctx, &notif.QueryEvent{
				Type:      notif.FinalPeer,
				Responses: []*pstore.PeerInfo{&pi},
			})
		}()
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func() func(cmds.Response) (io.Reader, error) {
			pfm := pfuncMap{
				notif.FinalPeer: func(obj *notif.QueryEvent, out io.Writer, verbose bool) {
					pi := obj.Responses[0]
					for _, a := range pi.Addrs {
						fmt.Fprintf(out, "%s\n", a)
					}
				},
			}

			return func(res cmds.Response) (io.Reader, error) {
				verbose, _, _ := res.Request().Option(dhtVerboseOptionName).Bool()
				v, err := unwrapOutput(res.Output())
				if err != nil {
					return nil, err
				}

				obj, ok := v.(*notif.QueryEvent)
				if !ok {
					return nil, e.TypeErr(obj, v)
				}

				buf := new(bytes.Buffer)
				printEvent(obj, buf, verbose, pfm)

				return buf, nil
			}
		}(),
	},
	Type: notif.QueryEvent{},
}

var getValueDhtCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Given a key, query the routing system for its best value.",
		ShortDescription: `
Outputs the best value for the given key.

There may be several different values for a given key stored in the routing
system; in this context 'best' means the record that is most desirable. There is
no one metric for 'best': it depends entirely on the key type. For IPNS, 'best'
is the record that is both valid and has the highest sequence number (freshest).
Different key types can specify other 'best' rules.
`,
	},

	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("key", true, true, "The key to find a value for."),
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption("verbose", dhtVerboseOptionName, "Print extra information."),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		if n.Routing == nil {
			res.SetError(ErrNotOnline, cmdkit.ErrNormal)
			return
		}

		dhtkey, err := escapeDhtKey(req.Arguments()[0])
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		outChan := make(chan interface{})
		res.SetOutput((<-chan interface{})(outChan))

		ctx, cancel := context.WithCancel(req.Context())
		ctx, events := notif.RegisterForQueryEvents(ctx)

		go func() {
			defer close(outChan)
			for e := range events {
				select {
				case outChan <- e:
				case <-req.Context().Done():
				}
			}
		}()

		go func() {
			defer cancel()
			val, err := n.Routing.GetValue(ctx, dhtkey)
			if err != nil {
				notif.PublishQueryEvent(ctx, &notif.QueryEvent{
					Type:  notif.QueryError,
					Extra: err.Error(),
				})
			} else {
				notif.PublishQueryEvent(ctx, &notif.QueryEvent{
					Type:  notif.Value,
					Extra: string(val),
				})
			}
		}()
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func() func(cmds.Response) (io.Reader, error) {
			pfm := pfuncMap{
				notif.Value: func(obj *notif.QueryEvent, out io.Writer, verbose bool) {
					if verbose {
						fmt.Fprintf(out, "got value: '%s'\n", obj.Extra)
					} else {
						fmt.Fprintln(out, obj.Extra)
					}
				},
			}

			return func(res cmds.Response) (io.Reader, error) {
				verbose, _, _ := res.Request().Option(dhtVerboseOptionName).Bool()
				v, err := unwrapOutput(res.Output())
				if err != nil {
					return nil, err
				}

				obj, ok := v.(*notif.QueryEvent)
				if !ok {
					return nil, e.TypeErr(obj, v)
				}

				buf := new(bytes.Buffer)

				printEvent(obj, buf, verbose, pfm)

				return buf, nil
			}
		}(),
	},
	Type: notif.QueryEvent{},
}

var putValueDhtCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Write a key/value pair to the routing system.",
		ShortDescription: `
Given a key of the form /foo/bar and a value of any form, this will write that
value to the routing system with that key.

Keys have two parts: a keytype (foo) and the key name (bar). IPNS uses the
/ipns keytype, and expects the key name to be a Peer ID. IPNS entries are
specifically formatted (protocol buffer).

You may only use keytypes that are supported in your ipfs binary: currently
this is only /ipns. Unless you have a relatively deep understanding of the
go-ipfs routing internals, you likely want to be using 'ipfs name publish' instead
of this.

Value is arbitrary text. Standard input can be used to provide value.

NOTE: A value may not exceed 2048 bytes.
`,
	},

	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("key", true, false, "The key to store the value at."),
		cmdkit.StringArg("value", true, false, "The value to store.").EnableStdin(),
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption("verbose", dhtVerboseOptionName, "Print extra information."),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		if n.Routing == nil {
			res.SetError(ErrNotOnline, cmdkit.ErrNormal)
			return
		}

		key, err := escapeDhtKey(req.Arguments()[0])
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		outChan := make(chan interface{})
		res.SetOutput((<-chan interface{})(outChan))

		data := req.Arguments()[1]

		ctx, cancel := context.WithCancel(req.Context())
		ctx, events := notif.RegisterForQueryEvents(ctx)

		go func() {
			defer close(outChan)
			for e := range events {
				select {
				case outChan <- e:
				case <-req.Context().Done():
					return
				}
			}
		}()

		go func() {
			defer cancel()
			err := n.Routing.PutValue(ctx, key, []byte(data))
			if err != nil {
				notif.PublishQueryEvent(ctx, &notif.QueryEvent{
					Type:  notif.QueryError,
					Extra: err.Error(),
				})
			}
		}()
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func() func(cmds.Response) (io.Reader, error) {
			pfm := pfuncMap{
				notif.FinalPeer: func(obj *notif.QueryEvent, out io.Writer, verbose bool) {
					if verbose {
						fmt.Fprintf(out, "* closest peer %s\n", obj.ID)
					}
				},
				notif.Value: func(obj *notif.QueryEvent, out io.Writer, verbose bool) {
					fmt.Fprintf(out, "%s\n", obj.ID.Pretty())
				},
			}

			return func(res cmds.Response) (io.Reader, error) {
				verbose, _, _ := res.Request().Option(dhtVerboseOptionName).Bool()
				v, err := unwrapOutput(res.Output())
				if err != nil {
					return nil, err
				}
				obj, ok := v.(*notif.QueryEvent)
				if !ok {
					return nil, e.TypeErr(obj, v)
				}

				buf := new(bytes.Buffer)
				printEvent(obj, buf, verbose, pfm)

				return buf, nil
			}
		}(),
	},
	Type: notif.QueryEvent{},
}

type printFunc func(obj *notif.QueryEvent, out io.Writer, verbose bool)
type pfuncMap map[notif.QueryEventType]printFunc

func printEvent(obj *notif.QueryEvent, out io.Writer, verbose bool, override pfuncMap) {
	if verbose {
		fmt.Fprintf(out, "%s: ", time.Now().Format("15:04:05.000"))
	}

	if override != nil {
		if pf, ok := override[obj.Type]; ok {
			pf(obj, out, verbose)
			return
		}
	}

	switch obj.Type {
	case notif.SendingQuery:
		if verbose {
			fmt.Fprintf(out, "* querying %s\n", obj.ID)
		}
	case notif.Value:
		if verbose {
			fmt.Fprintf(out, "got value: '%s'\n", obj.Extra)
		} else {
			fmt.Fprint(out, obj.Extra)
		}
	case notif.PeerResponse:
		if verbose {
			fmt.Fprintf(out, "* %s says use ", obj.ID)
			for _, p := range obj.Responses {
				fmt.Fprintf(out, "%s ", p.ID)
			}
			fmt.Fprintln(out)
		}
	case notif.QueryError:
		if verbose {
			fmt.Fprintf(out, "error: %s\n", obj.Extra)
		}
	case notif.DialingPeer:
		if verbose {
			fmt.Fprintf(out, "dialing peer: %s\n", obj.ID)
		}
	case notif.AddingPeer:
		if verbose {
			fmt.Fprintf(out, "adding peer to query: %s\n", obj.ID)
		}
	case notif.FinalPeer:
	default:
		if verbose {
			fmt.Fprintf(out, "unrecognized event type: %d\n", obj.Type)
		}
	}
}

func escapeDhtKey(s string) (string, error) {
	parts := path.SplitList(s)
	switch len(parts) {
	case 1:
		k, err := b58.Decode(s)
		if err != nil {
			return "", err
		}
		return string(k), nil
	case 3:
		k, err := b58.Decode(parts[2])
		if err != nil {
			return "", err
		}
		return path.Join(append(parts[:2], string(k))), nil
	default:
		return "", errors.New("invalid key")
	}
}
