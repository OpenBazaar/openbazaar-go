package name

import (
	"errors"
	"fmt"
	"io"
	"time"

	cmdenv "github.com/ipfs/go-ipfs/core/commands/cmdenv"
	e "github.com/ipfs/go-ipfs/core/commands/e"
	iface "github.com/ipfs/go-ipfs/core/coreapi/interface"
	options "github.com/ipfs/go-ipfs/core/coreapi/interface/options"

	cmds "gx/ipfs/QmSXUokcP4TJpFfqozT69AVAYRtzXVMUjzQVkYX41R9Svs/go-ipfs-cmds"
	"gx/ipfs/Qmde5VP1qUkyQXKCfmEUA7bP64V2HAptbJ7phuPp7jXWwg/go-ipfs-cmdkit"
)

var (
	errAllowOffline = errors.New("can't publish while offline: pass `--allow-offline` to override")
)

const (
	ipfsPathOptionName     = "ipfs-path"
	resolveOptionName      = "resolve"
	allowOfflineOptionName = "allow-offline"
	lifeTimeOptionName     = "lifetime"
	ttlOptionName          = "ttl"
	keyOptionName          = "key"
	quieterOptionName      = "quieter"
)

var PublishCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Publish IPNS names.",
		ShortDescription: `
IPNS is a PKI namespace, where names are the hashes of public keys, and
the private key enables publishing new (signed) values. In both publish
and resolve, the default name used is the node's own PeerID,
which is the hash of its public key.
`,
		LongDescription: `
IPNS is a PKI namespace, where names are the hashes of public keys, and
the private key enables publishing new (signed) values. In both publish
and resolve, the default name used is the node's own PeerID,
which is the hash of its public key.

You can use the 'ipfs key' commands to list and generate more names and their
respective keys.

Examples:

Publish an <ipfs-path> with your default name:

  > ipfs name publish /ipfs/QmatmE9msSfkKxoffpHwNLNKgwZG8eT9Bud6YoPab52vpy
  Published to QmbCMUZw6JFeZ7Wp9jkzbye3Fzp2GGcPgC3nmeUjfVF87n: /ipfs/QmatmE9msSfkKxoffpHwNLNKgwZG8eT9Bud6YoPab52vpy

Publish an <ipfs-path> with another name, added by an 'ipfs key' command:

  > ipfs key gen --type=rsa --size=2048 mykey
  > ipfs name publish --key=mykey /ipfs/QmatmE9msSfkKxoffpHwNLNKgwZG8eT9Bud6YoPab52vpy
  Published to QmSrPmbaUKA3ZodhzPWZnpFgcPMFWF4QsxXbkWfEptTBJd: /ipfs/QmatmE9msSfkKxoffpHwNLNKgwZG8eT9Bud6YoPab52vpy

Alternatively, publish an <ipfs-path> using a valid PeerID (as listed by 
'ipfs key list -l'):

 > ipfs name publish --key=QmbCMUZw6JFeZ7Wp9jkzbye3Fzp2GGcPgC3nmeUjfVF87n /ipfs/QmatmE9msSfkKxoffpHwNLNKgwZG8eT9Bud6YoPab52vpy
  Published to QmbCMUZw6JFeZ7Wp9jkzbye3Fzp2GGcPgC3nmeUjfVF87n: /ipfs/QmatmE9msSfkKxoffpHwNLNKgwZG8eT9Bud6YoPab52vpy

`,
	},

	Arguments: []cmdkit.Argument{
		cmdkit.StringArg(ipfsPathOptionName, true, false, "ipfs path of the object to be published.").EnableStdin(),
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption(resolveOptionName, "Resolve given path before publishing.").WithDefault(true),
		cmdkit.StringOption(lifeTimeOptionName, "t",
			`Time duration that the record will be valid for. <<default>>
    This accepts durations such as "300s", "1.5h" or "2h45m". Valid time units are
    "ns", "us" (or "µs"), "ms", "s", "m", "h".`).WithDefault("24h"),
		cmdkit.BoolOption(allowOfflineOptionName, "When offline, save the IPNS record to the the local datastore without broadcasting to the network instead of simply failing."),
		cmdkit.StringOption(ttlOptionName, "Time duration this record should be cached for (caution: experimental)."),
		cmdkit.StringOption(keyOptionName, "k", "Name of the key to be used or a valid PeerID, as listed by 'ipfs key list -l'. Default: <<default>>.").WithDefault("self"),
		cmdkit.BoolOption(quieterOptionName, "Q", "Write only final hash."),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env)
		if err != nil {
			return err
		}

		allowOffline, _ := req.Options[allowOfflineOptionName].(bool)
		kname, _ := req.Options[keyOptionName].(string)

		validTimeOpt, _ := req.Options[lifeTimeOptionName].(string)
		validTime, err := time.ParseDuration(validTimeOpt)
		if err != nil {
			return fmt.Errorf("error parsing lifetime option: %s", err)
		}

		opts := []options.NamePublishOption{
			options.Name.AllowOffline(allowOffline),
			options.Name.Key(kname),
			options.Name.ValidTime(validTime),
		}

		if ttl, found := req.Options[ttlOptionName].(string); found {
			d, err := time.ParseDuration(ttl)
			if err != nil {
				return err
			}

			opts = append(opts, options.Name.TTL(d))
		}

		p, err := iface.ParsePath(req.Arguments[0])
		if err != nil {
			return err
		}

		if verifyExists, _ := req.Options[resolveOptionName].(bool); verifyExists {
			_, err := api.ResolveNode(req.Context, p)
			if err != nil {
				return err
			}
		}

		out, err := api.Name().Publish(req.Context, p, opts...)
		if err != nil {
			if err == iface.ErrOffline {
				err = errAllowOffline
			}
			return err
		}

		return cmds.EmitOnce(res, &IpnsEntry{
			Name:  out.Name(),
			Value: out.Value().String(),
		})
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeEncoder(func(req *cmds.Request, w io.Writer, v interface{}) error {
			entry, ok := v.(*IpnsEntry)
			if !ok {
				return e.TypeErr(entry, v)
			}

			var err error
			quieter, _ := req.Options[quieterOptionName].(bool)
			if quieter {
				_, err = fmt.Fprintln(w, entry.Name)
			} else {
				_, err = fmt.Fprintf(w, "Published to %s: %s\n", entry.Name, entry.Value)
			}
			return err
		}),
	},
	Type: IpnsEntry{},
}
