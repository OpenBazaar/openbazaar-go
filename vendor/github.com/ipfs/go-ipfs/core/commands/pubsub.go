package commands

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net/http"
	"sort"

	cmdenv "github.com/ipfs/go-ipfs/core/commands/cmdenv"
	e "github.com/ipfs/go-ipfs/core/commands/e"
	options "github.com/ipfs/go-ipfs/core/coreapi/interface/options"

	cmds "gx/ipfs/QmSXUokcP4TJpFfqozT69AVAYRtzXVMUjzQVkYX41R9Svs/go-ipfs-cmds"
	cmdkit "gx/ipfs/Qmde5VP1qUkyQXKCfmEUA7bP64V2HAptbJ7phuPp7jXWwg/go-ipfs-cmdkit"
)

var PubsubCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "An experimental publish-subscribe system on ipfs.",
		ShortDescription: `
ipfs pubsub allows you to publish messages to a given topic, and also to
subscribe to new messages on a given topic.

This is an experimental feature. It is not intended in its current state
to be used in a production environment.

To use, the daemon must be run with '--enable-pubsub-experiment'.
`,
	},
	Subcommands: map[string]*cmds.Command{
		"pub":   PubsubPubCmd,
		"sub":   PubsubSubCmd,
		"ls":    PubsubLsCmd,
		"peers": PubsubPeersCmd,
	},
}

const (
	pubsubDiscoverOptionName = "discover"
)

type pubsubMessage struct {
	From     []byte   `json:"from,omitempty"`
	Data     []byte   `json:"data,omitempty"`
	Seqno    []byte   `json:"seqno,omitempty"`
	TopicIDs []string `json:"topicIDs,omitempty"`
}

var PubsubSubCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Subscribe to messages on a given topic.",
		ShortDescription: `
ipfs pubsub sub subscribes to messages on a given topic.

This is an experimental feature. It is not intended in its current state
to be used in a production environment.

To use, the daemon must be run with '--enable-pubsub-experiment'.
`,
		LongDescription: `
ipfs pubsub sub subscribes to messages on a given topic.

This is an experimental feature. It is not intended in its current state
to be used in a production environment.

To use, the daemon must be run with '--enable-pubsub-experiment'.

This command outputs data in the following encodings:
  * "json"
(Specified by the "--encoding" or "--enc" flag)
`,
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("topic", true, false, "String name of topic to subscribe to."),
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption(pubsubDiscoverOptionName, "try to discover other peers subscribed to the same topic"),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env)
		if err != nil {
			return err
		}

		topic := req.Arguments[0]
		discover, _ := req.Options[pubsubDiscoverOptionName].(bool)

		sub, err := api.PubSub().Subscribe(req.Context, topic, options.PubSub.Discover(discover))
		if err != nil {
			return err
		}
		defer sub.Close()

		if f, ok := res.(http.Flusher); ok {
			f.Flush()
		}

		for {
			msg, err := sub.Next(req.Context)
			if err == io.EOF || err == context.Canceled {
				return nil
			} else if err != nil {
				return err
			}

			if err := res.Emit(&pubsubMessage{
				Data:     msg.Data(),
				From:     []byte(msg.From()),
				Seqno:    msg.Seq(),
				TopicIDs: msg.Topics(),
			}); err != nil {
				return err
			}
		}
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeEncoder(func(req *cmds.Request, w io.Writer, v interface{}) error {
			m, ok := v.(*pubsubMessage)
			if !ok {
				return fmt.Errorf("unexpected type: %T", v)
			}

			_, err := w.Write(m.Data)
			return err
		}),
		"ndpayload": cmds.MakeEncoder(func(req *cmds.Request, w io.Writer, v interface{}) error {
			m, ok := v.(*pubsubMessage)
			if !ok {
				return fmt.Errorf("unexpected type: %T", v)
			}

			m.Data = append(m.Data, '\n')
			_, err := w.Write(m.Data)
			return err
		}),
		"lenpayload": cmds.MakeEncoder(func(req *cmds.Request, w io.Writer, v interface{}) error {
			m, ok := v.(*pubsubMessage)
			if !ok {
				return fmt.Errorf("unexpected type: %T", v)
			}

			buf := make([]byte, 8, len(m.Data)+8)

			n := binary.PutUvarint(buf, uint64(len(m.Data)))
			buf = append(buf[:n], m.Data...)
			_, err := w.Write(buf)
			return err
		}),
	},
	Type: pubsubMessage{},
}

var PubsubPubCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Publish a message to a given pubsub topic.",
		ShortDescription: `
ipfs pubsub pub publishes a message to a specified topic.

This is an experimental feature. It is not intended in its current state
to be used in a production environment.

To use, the daemon must be run with '--enable-pubsub-experiment'.
`,
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("topic", true, false, "Topic to publish to."),
		cmdkit.StringArg("data", true, true, "Payload of message to publish.").EnableStdin(),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env)
		if err != nil {
			return err
		}

		topic := req.Arguments[0]

		err = req.ParseBodyArgs()
		if err != nil {
			return err
		}

		for _, data := range req.Arguments[1:] {
			if err := api.PubSub().Publish(req.Context, topic, []byte(data)); err != nil {
				return err
			}
		}

		return nil
	},
}

var PubsubLsCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "List subscribed topics by name.",
		ShortDescription: `
ipfs pubsub ls lists out the names of topics you are currently subscribed to.

This is an experimental feature. It is not intended in its current state
to be used in a production environment.

To use, the daemon must be run with '--enable-pubsub-experiment'.
`,
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env)
		if err != nil {
			return err
		}

		l, err := api.PubSub().Ls(req.Context)
		if err != nil {
			return err
		}

		return cmds.EmitOnce(res, stringList{l})
	},
	Type: stringList{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeEncoder(stringListEncoder),
	},
}

func stringListEncoder(req *cmds.Request, w io.Writer, v interface{}) error {
	list, ok := v.(*stringList)
	if !ok {
		return e.TypeErr(list, v)
	}
	for _, str := range list.Strings {
		_, err := fmt.Fprintf(w, "%s\n", str)
		if err != nil {
			return err
		}
	}
	return nil
}

var PubsubPeersCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "List peers we are currently pubsubbing with.",
		ShortDescription: `
ipfs pubsub peers with no arguments lists out the pubsub peers you are
currently connected to. If given a topic, it will list connected
peers who are subscribed to the named topic.

This is an experimental feature. It is not intended in its current state
to be used in a production environment.

To use, the daemon must be run with '--enable-pubsub-experiment'.
`,
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("topic", false, false, "topic to list connected peers of"),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env)
		if err != nil {
			return err
		}

		var topic string
		if len(req.Arguments) == 1 {
			topic = req.Arguments[0]
		}

		peers, err := api.PubSub().Peers(req.Context, options.PubSub.Topic(topic))
		if err != nil {
			return err
		}

		list := &stringList{make([]string, 0, len(peers))}

		for _, peer := range peers {
			list.Strings = append(list.Strings, peer.Pretty())
		}
		sort.Strings(list.Strings)
		return cmds.EmitOnce(res, list)
	},
	Type: stringList{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeEncoder(stringListEncoder),
	},
}
