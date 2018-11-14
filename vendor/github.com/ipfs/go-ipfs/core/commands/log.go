package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	cmds "github.com/ipfs/go-ipfs/commands"
	e "github.com/ipfs/go-ipfs/core/commands/e"

	logging "gx/ipfs/QmZChCsSt8DctjceaL56Eibc29CVQq4dGKRXC5JRZ6Ppae/go-log"
	lwriter "gx/ipfs/QmZChCsSt8DctjceaL56Eibc29CVQq4dGKRXC5JRZ6Ppae/go-log/writer"
	"gx/ipfs/Qmde5VP1qUkyQXKCfmEUA7bP64V2HAptbJ7phuPp7jXWwg/go-ipfs-cmdkit"
)

// Golang os.Args overrides * and replaces the character argument with
// an array which includes every file in the user's CWD. As a
// workaround, we use 'all' instead. The util library still uses * so
// we convert it at this step.
var logAllKeyword = "all"

var LogCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Interact with the daemon log output.",
		ShortDescription: `
'ipfs log' contains utility commands to affect or read the logging
output of a running daemon.
`,
	},

	Subcommands: map[string]*cmds.Command{
		"level": logLevelCmd,
		"ls":    logLsCmd,
		"tail":  logTailCmd,
	},
}

var logLevelCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Change the logging level.",
		ShortDescription: `
Change the verbosity of one or all subsystems log output. This does not affect
the event log.
`,
	},

	Arguments: []cmdkit.Argument{
		// TODO use a different keyword for 'all' because all can theoretically
		// clash with a subsystem name
		cmdkit.StringArg("subsystem", true, false, fmt.Sprintf("The subsystem logging identifier. Use '%s' for all subsystems.", logAllKeyword)),
		cmdkit.StringArg("level", true, false, `The log level, with 'debug' the most verbose and 'critical' the least verbose.
			One of: debug, info, warning, error, critical.
		`),
	},
	Run: func(req cmds.Request, res cmds.Response) {

		args := req.Arguments()
		subsystem, level := args[0], args[1]

		if subsystem == logAllKeyword {
			subsystem = "*"
		}

		if err := logging.SetLogLevel(subsystem, level); err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		s := fmt.Sprintf("Changed log level of '%s' to '%s'\n", subsystem, level)
		log.Info(s)
		res.SetOutput(&MessageOutput{s})
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: MessageTextMarshaler,
	},
	Type: MessageOutput{},
}

var logLsCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "List the logging subsystems.",
		ShortDescription: `
'ipfs log ls' is a utility command used to list the logging
subsystems of a running daemon.
`,
	},
	Run: func(req cmds.Request, res cmds.Response) {
		res.SetOutput(&stringList{logging.GetSubsystems()})
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: stringListMarshaler,
	},
	Type: stringList{},
}

var logTailCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Read the event log.",
		ShortDescription: `
Outputs event log messages (not other log messages) as they are generated.
`,
	},

	Run: func(req cmds.Request, res cmds.Response) {
		ctx := req.Context()
		r1, w1 := io.Pipe()
		r2, w2 := io.Pipe()
		go func() {
			defer w1.Close()
			<-ctx.Done()
		}()
		// Reformat the logs as ndjson
		// TODO: remove this: #5709
		go func() {
			defer w2.Close()
			decoder := json.NewDecoder(r1)
			encoder := json.NewEncoder(w2)
			for {
				var obj interface{}
				if decoder.Decode(&obj) != nil || encoder.Encode(obj) != nil {
					return
				}
			}
		}()

		lwriter.WriterGroup.AddWriter(w1)
		res.SetOutput(r2)
	},
}

func stringListMarshaler(res cmds.Response) (io.Reader, error) {
	v, err := unwrapOutput(res.Output())
	if err != nil {
		return nil, err
	}

	list, ok := v.(*stringList)
	if !ok {
		return nil, e.TypeErr(list, v)
	}

	buf := new(bytes.Buffer)
	for _, s := range list.Strings {
		buf.WriteString(s)
		buf.WriteString("\n")
	}

	return buf, nil
}
