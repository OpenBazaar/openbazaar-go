package unixfs

import (
	"bytes"
	"fmt"
	"io"
	"sort"
	"text/tabwriter"

	cmds "github.com/ipfs/go-ipfs/commands"
	e "github.com/ipfs/go-ipfs/core/commands/e"
	iface "github.com/ipfs/go-ipfs/core/coreapi/interface"

	merkledag "gx/ipfs/QmSei8kFMfqdJq7Q68d2LMnHbTWKKg2daA29ezUYFAUNgc/go-merkledag"
	cmdkit "gx/ipfs/Qmde5VP1qUkyQXKCfmEUA7bP64V2HAptbJ7phuPp7jXWwg/go-ipfs-cmdkit"
	unixfs "gx/ipfs/QmfB3oNXGGq9S4B2a9YeCajoATms3Zw2VvDm8fK7VeLSV8/go-unixfs"
)

type LsLink struct {
	Name, Hash string
	Size       uint64
	Type       string
}

type LsObject struct {
	Hash  string
	Size  uint64
	Type  string
	Links []LsLink
}

type LsOutput struct {
	Arguments map[string]string
	Objects   map[string]*LsObject
}

var LsCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "List directory contents for Unix filesystem objects.",
		ShortDescription: `
Displays the contents of an IPFS or IPNS object(s) at the given path.

The JSON output contains size information. For files, the child size
is the total size of the file contents. For directories, the child
size is the IPFS link size.

This functionality is deprecated, and will be removed in future versions. If
possible, please use 'ipfs ls' instead.
`,
		LongDescription: `
Displays the contents of an IPFS or IPNS object(s) at the given path.

The JSON output contains size information. For files, the child size
is the total size of the file contents. For directories, the child
size is the IPFS link size.

The path can be a prefixless ref; in this case, we assume it to be an
/ipfs ref and not /ipns.

Example:

    > ipfs file ls QmW2WQi7j6c7UgJTarActp7tDNikE4B2qXtFCfLPdsgaTQ
    cat.jpg
    > ipfs file ls /ipfs/QmW2WQi7j6c7UgJTarActp7tDNikE4B2qXtFCfLPdsgaTQ
    cat.jpg

This functionality is deprecated, and will be removed in future versions. If
possible, please use 'ipfs ls' instead.
`,
	},

	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("ipfs-path", true, true, "The path to the IPFS object(s) to list links from.").EnableStdin(),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		node, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		api, err := req.InvocContext().GetApi()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		paths := req.Arguments()

		output := LsOutput{
			Arguments: map[string]string{},
			Objects:   map[string]*LsObject{},
		}

		for _, p := range paths {
			ctx := req.Context()

			fpath, err := iface.ParsePath(p)
			if err != nil {
				res.SetError(err, cmdkit.ErrNormal)
				return
			}

			merkleNode, err := api.ResolveNode(ctx, fpath)
			if err != nil {
				res.SetError(err, cmdkit.ErrNormal)
				return
			}

			c := merkleNode.Cid()

			hash := c.String()
			output.Arguments[p] = hash

			if _, ok := output.Objects[hash]; ok {
				// duplicate argument for an already-listed node
				continue
			}

			ndpb, ok := merkleNode.(*merkledag.ProtoNode)
			if !ok {
				res.SetError(merkledag.ErrNotProtobuf, cmdkit.ErrNormal)
				return
			}

			unixFSNode, err := unixfs.FSNodeFromBytes(ndpb.Data())
			if err != nil {
				res.SetError(err, cmdkit.ErrNormal)
				return
			}

			t := unixFSNode.Type()

			output.Objects[hash] = &LsObject{
				Hash: c.String(),
				Type: t.String(),
				Size: unixFSNode.FileSize(),
			}

			switch t {
			case unixfs.TFile:
				break
			case unixfs.THAMTShard:
				// We need a streaming ls API for this.
				res.SetError(fmt.Errorf("cannot list large directories yet"), cmdkit.ErrNormal)
				return
			case unixfs.TDirectory:
				links := make([]LsLink, len(merkleNode.Links()))
				output.Objects[hash].Links = links
				for i, link := range merkleNode.Links() {
					linkNode, err := link.GetNode(ctx, node.DAG)
					if err != nil {
						res.SetError(err, cmdkit.ErrNormal)
						return
					}
					lnpb, ok := linkNode.(*merkledag.ProtoNode)
					if !ok {
						res.SetError(merkledag.ErrNotProtobuf, cmdkit.ErrNormal)
						return
					}

					d, err := unixfs.FSNodeFromBytes(lnpb.Data())
					if err != nil {
						res.SetError(err, cmdkit.ErrNormal)
						return
					}
					t := d.Type()
					lsLink := LsLink{
						Name: link.Name,
						Hash: link.Cid.String(),
						Type: t.String(),
					}
					if t == unixfs.TFile {
						lsLink.Size = d.FileSize()
					} else {
						lsLink.Size = link.Size
					}
					links[i] = lsLink
				}
			case unixfs.TSymlink:
				res.SetError(fmt.Errorf("cannot list symlinks yet"), cmdkit.ErrNormal)
				return
			default:
				res.SetError(fmt.Errorf("unrecognized type: %s", t), cmdkit.ErrImplementation)
				return
			}
		}

		res.SetOutput(&output)
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			v, err := unwrapOutput(res.Output())
			if err != nil {
				return nil, err
			}

			output, ok := v.(*LsOutput)
			if !ok {
				return nil, e.TypeErr(output, v)
			}
			buf := new(bytes.Buffer)
			w := tabwriter.NewWriter(buf, 1, 2, 1, ' ', 0)

			nonDirectories := []string{}
			directories := []string{}
			for argument, hash := range output.Arguments {
				object, ok := output.Objects[hash]
				if !ok {
					return nil, fmt.Errorf("unresolved hash: %s", hash)
				}

				if object.Type == "Directory" {
					directories = append(directories, argument)
				} else {
					nonDirectories = append(nonDirectories, argument)
				}
			}
			sort.Strings(nonDirectories)
			sort.Strings(directories)

			for _, argument := range nonDirectories {
				fmt.Fprintf(w, "%s\n", argument)
			}

			seen := map[string]bool{}
			for i, argument := range directories {
				hash := output.Arguments[argument]
				if _, ok := seen[hash]; ok {
					continue
				}
				seen[hash] = true

				object := output.Objects[hash]
				if i > 0 || len(nonDirectories) > 0 {
					fmt.Fprintln(w)
				}
				if len(output.Arguments) > 1 {
					for _, arg := range directories[i:] {
						if output.Arguments[arg] == hash {
							fmt.Fprintf(w, "%s:\n", arg)
						}
					}
				}
				for _, link := range object.Links {
					fmt.Fprintf(w, "%s\n", link.Name)
				}
			}
			w.Flush()

			return buf, nil
		},
	},
	Type: LsOutput{},
}
