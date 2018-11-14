package commands

import (
	"fmt"
	"io"
	"os"
	"time"

	cmdenv "github.com/ipfs/go-ipfs/core/commands/cmdenv"

	humanize "gx/ipfs/QmPSBJL4momYnE7DcUyk2DVhD6rH488ZmHBGLbxNdhU44K/go-humanize"
	cmds "gx/ipfs/QmSXUokcP4TJpFfqozT69AVAYRtzXVMUjzQVkYX41R9Svs/go-ipfs-cmds"
	peer "gx/ipfs/QmTRhk7cgjUf2gfQ3p2M9KPECNZEW9XUrmHcFCgog4cPgB/go-libp2p-peer"
	protocol "gx/ipfs/QmZNkThpqfVXs9GNbexPrfBbXSLNYeKrE7jwFM2oqHbyqN/go-libp2p-protocol"
	cmdkit "gx/ipfs/Qmde5VP1qUkyQXKCfmEUA7bP64V2HAptbJ7phuPp7jXWwg/go-ipfs-cmdkit"
	metrics "gx/ipfs/QmeaTjsfPf6vQ3WU2BUdjakgvKUHpuv3Fjxvb75N5iksMx/go-libp2p-metrics"
)

var StatsCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Query IPFS statistics.",
		ShortDescription: `'ipfs stats' is a set of commands to help look at statistics
for your IPFS node.
`,
		LongDescription: `'ipfs stats' is a set of commands to help look at statistics
for your IPFS node.`,
	},

	Subcommands: map[string]*cmds.Command{
		"bw":      statBwCmd,
		"repo":    repoStatCmd,
		"bitswap": bitswapStatCmd,
	},
}

const (
	statPeerOptionName     = "peer"
	statProtoOptionName    = "proto"
	statPollOptionName     = "poll"
	statIntervalOptionName = "interval"
)

var statBwCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Print ipfs bandwidth information.",
		ShortDescription: `'ipfs stats bw' prints bandwidth information for the ipfs daemon.
It displays: TotalIn, TotalOut, RateIn, RateOut.
		`,
		LongDescription: `'ipfs stats bw' prints bandwidth information for the ipfs daemon.
It displays: TotalIn, TotalOut, RateIn, RateOut.

By default, overall bandwidth and all protocols are shown. To limit bandwidth
to a particular peer, use the 'peer' option along with that peer's multihash
id. To specify a specific protocol, use the 'proto' option. The 'peer' and
'proto' options cannot be specified simultaneously. The protocols that are
queried using this method are outlined in the specification:
https://github.com/libp2p/specs/blob/master/7-properties.md#757-protocol-multicodecs

Example protocol options:
  - /ipfs/id/1.0.0
  - /ipfs/bitswap
  - /ipfs/dht

Example:

    > ipfs stats bw -t /ipfs/bitswap
    Bandwidth
    TotalIn: 5.0MB
    TotalOut: 0B
    RateIn: 343B/s
    RateOut: 0B/s
    > ipfs stats bw -p QmepgFW7BHEtU4pZJdxaNiv75mKLLRQnPi1KaaXmQN4V1a
    Bandwidth
    TotalIn: 4.9MB
    TotalOut: 12MB
    RateIn: 0B/s
    RateOut: 0B/s
`,
	},
	Options: []cmdkit.Option{
		cmdkit.StringOption(statPeerOptionName, "p", "Specify a peer to print bandwidth for."),
		cmdkit.StringOption(statProtoOptionName, "t", "Specify a protocol to print bandwidth for."),
		cmdkit.BoolOption(statPollOptionName, "Print bandwidth at an interval."),
		cmdkit.StringOption(statIntervalOptionName, "i", `Time interval to wait between updating output, if 'poll' is true.

    This accepts durations such as "300s", "1.5h" or "2h45m". Valid time units are:
    "ns", "us" (or "µs"), "ms", "s", "m", "h".`).WithDefault("1s"),
	},

	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		nd, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		// Must be online!
		if !nd.OnlineMode() {
			return cmdkit.Errorf(cmdkit.ErrClient, ErrNotOnline.Error())
		}

		if nd.Reporter == nil {
			return fmt.Errorf("bandwidth reporter disabled in config")
		}

		pstr, pfound := req.Options[statPeerOptionName].(string)
		tstr, tfound := req.Options["proto"].(string)
		if pfound && tfound {
			return cmdkit.Errorf(cmdkit.ErrClient, "please only specify peer OR protocol")
		}

		var pid peer.ID
		if pfound {
			checkpid, err := peer.IDB58Decode(pstr)
			if err != nil {
				return err
			}
			pid = checkpid
		}

		timeS, _ := req.Options[statIntervalOptionName].(string)
		interval, err := time.ParseDuration(timeS)
		if err != nil {
			return err
		}

		doPoll, _ := req.Options[statPollOptionName].(bool)
		for {
			if pfound {
				stats := nd.Reporter.GetBandwidthForPeer(pid)
				if err := res.Emit(&stats); err != nil {
					return err
				}
			} else if tfound {
				protoId := protocol.ID(tstr)
				stats := nd.Reporter.GetBandwidthForProtocol(protoId)
				if err := res.Emit(&stats); err != nil {
					return err
				}
			} else {
				totals := nd.Reporter.GetBandwidthTotals()
				if err := res.Emit(&totals); err != nil {
					return err
				}
			}
			if !doPoll {
				return nil
			}
			select {
			case <-time.After(interval):
			case <-req.Context.Done():
				return req.Context.Err()
			}
		}
	},
	Type: metrics.Stats{},
	PostRun: cmds.PostRunMap{
		cmds.CLI: func(res cmds.Response, re cmds.ResponseEmitter) error {
			polling, _ := res.Request().Options[statPollOptionName].(bool)

			if polling {
				fmt.Fprintln(os.Stdout, "Total Up    Total Down  Rate Up     Rate Down")
			}
			for {
				v, err := res.Next()
				if err != nil {
					if err == io.EOF {
						return nil
					}
					return err
				}

				bs := v.(*metrics.Stats)

				if !polling {
					printStats(os.Stdout, bs)
					return nil
				}

				fmt.Fprintf(os.Stdout, "%8s    ", humanize.Bytes(uint64(bs.TotalOut)))
				fmt.Fprintf(os.Stdout, "%8s    ", humanize.Bytes(uint64(bs.TotalIn)))
				fmt.Fprintf(os.Stdout, "%8s/s  ", humanize.Bytes(uint64(bs.RateOut)))
				fmt.Fprintf(os.Stdout, "%8s/s      \r", humanize.Bytes(uint64(bs.RateIn)))
			}
		},
	},
}

func printStats(out io.Writer, bs *metrics.Stats) {
	fmt.Fprintln(out, "Bandwidth")
	fmt.Fprintf(out, "TotalIn: %s\n", humanize.Bytes(uint64(bs.TotalIn)))
	fmt.Fprintf(out, "TotalOut: %s\n", humanize.Bytes(uint64(bs.TotalOut)))
	fmt.Fprintf(out, "RateIn: %s/s\n", humanize.Bytes(uint64(bs.RateIn)))
	fmt.Fprintf(out, "RateOut: %s/s\n", humanize.Bytes(uint64(bs.RateOut)))
}
