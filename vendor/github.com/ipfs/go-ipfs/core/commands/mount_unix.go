// +build !windows,!nofuse

package commands

import (
	"fmt"
	"io"
	"strings"

	cmds "github.com/ipfs/go-ipfs/commands"
	e "github.com/ipfs/go-ipfs/core/commands/e"
	nodeMount "github.com/ipfs/go-ipfs/fuse/node"

	config "gx/ipfs/QmPEpj17FDRpc7K1aArKZp3RsHtzRMKykeK9GVgn4WQGPR/go-ipfs-config"
	"gx/ipfs/Qmde5VP1qUkyQXKCfmEUA7bP64V2HAptbJ7phuPp7jXWwg/go-ipfs-cmdkit"
)

var MountCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Mounts IPFS to the filesystem (read-only).",
		ShortDescription: `
Mount IPFS at a read-only mountpoint on the OS (default: /ipfs and /ipns).
All IPFS objects will be accessible under that directory. Note that the
root will not be listable, as it is virtual. Access known paths directly.

You may have to create /ipfs and /ipns before using 'ipfs mount':

> sudo mkdir /ipfs /ipns
> sudo chown $(whoami) /ipfs /ipns
> ipfs daemon &
> ipfs mount
`,
		LongDescription: `
Mount IPFS at a read-only mountpoint on the OS. The default, /ipfs and /ipns,
are set in the configuration file, but can be overriden by the options.
All IPFS objects will be accessible under this directory. Note that the
root will not be listable, as it is virtual. Access known paths directly.

You may have to create /ipfs and /ipns before using 'ipfs mount':

> sudo mkdir /ipfs /ipns
> sudo chown $(whoami) /ipfs /ipns
> ipfs daemon &
> ipfs mount

Example:

# setup
> mkdir foo
> echo "baz" > foo/bar
> ipfs add -r foo
added QmWLdkp93sNxGRjnFHPaYg8tCQ35NBY3XPn6KiETd3Z4WR foo/bar
added QmSh5e7S6fdcu75LAbXNZAFY2nGyZUJXyLCJDvn2zRkWyC foo
> ipfs ls QmSh5e7S6fdcu75LAbXNZAFY2nGyZUJXyLCJDvn2zRkWyC
QmWLdkp93sNxGRjnFHPaYg8tCQ35NBY3XPn6KiETd3Z4WR 12 bar
> ipfs cat QmWLdkp93sNxGRjnFHPaYg8tCQ35NBY3XPn6KiETd3Z4WR
baz

# mount
> ipfs daemon &
> ipfs mount
IPFS mounted at: /ipfs
IPNS mounted at: /ipns
> cd /ipfs/QmSh5e7S6fdcu75LAbXNZAFY2nGyZUJXyLCJDvn2zRkWyC
> ls
bar
> cat bar
baz
> cat /ipfs/QmSh5e7S6fdcu75LAbXNZAFY2nGyZUJXyLCJDvn2zRkWyC/bar
baz
> cat /ipfs/QmWLdkp93sNxGRjnFHPaYg8tCQ35NBY3XPn6KiETd3Z4WR
baz
`,
	},
	Options: []cmdkit.Option{
		cmdkit.StringOption("ipfs-path", "f", "The path where IPFS should be mounted."),
		cmdkit.StringOption("ipns-path", "n", "The path where IPNS should be mounted."),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		cfg, err := req.InvocContext().GetConfig()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		node, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		// error if we aren't running node in online mode
		if node.LocalMode() {
			res.SetError(ErrNotOnline, cmdkit.ErrClient)
			return
		}

		fsdir, found, err := req.Option("f").String()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}
		if !found {
			fsdir = cfg.Mounts.IPFS // use default value
		}

		// get default mount points
		nsdir, found, err := req.Option("n").String()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}
		if !found {
			nsdir = cfg.Mounts.IPNS // NB: be sure to not redeclare!
		}

		err = nodeMount.Mount(node, fsdir, nsdir)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		var output config.Mounts
		output.IPFS = fsdir
		output.IPNS = nsdir
		res.SetOutput(&output)
	},
	Type: config.Mounts{},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			v, err := unwrapOutput(res.Output())
			if err != nil {
				return nil, err
			}

			mnts, ok := v.(*config.Mounts)
			if !ok {
				return nil, e.TypeErr(mnts, v)
			}

			s := fmt.Sprintf("IPFS mounted at: %s\n", mnts.IPFS)
			s += fmt.Sprintf("IPNS mounted at: %s\n", mnts.IPNS)
			return strings.NewReader(s), nil
		},
	},
}
