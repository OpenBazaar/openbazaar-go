package ipfs

import (
	"github.com/op/go-logging"
	cmds "github.com/ipfs/go-ipfs/commands"
	commands "github.com/ipfs/go-ipfs/core/commands"
	cli "github.com/ipfs/go-ipfs/commands/cli"
	"gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"
)

var log = logging.MustGetLogger("ipfs")

var Root = &cmds.Command{
	Options:  commands.Root.Options,
	Helptext: commands.Root.Helptext,
}

var commandsClientCmd = commands.CommandsCmd(Root)

var localCommands = map[string]*cmds.Command{
	"commands": commandsClientCmd,
}

func init() {
	Root.Subcommands = localCommands
	for k, v := range commands.Root.Subcommands {
		if _, found := Root.Subcommands[k]; !found {
			Root.Subcommands[k] = v
		}
	}
}

func NewRequest(ctx cmds.Context, args []string) (cmds.Request, *cmds.Command, error) {
	Root.Subcommands = localCommands
	for k, v := range commands.Root.Subcommands {
		if _, found := Root.Subcommands[k]; !found {
			Root.Subcommands[k] = v
		}
	}
	req, cmd, _, err := cli.Parse(args, nil, Root)
	cctx := context.Background()
	rerr := req.SetRootContext(cctx)
	if rerr != nil {
		return nil, nil, rerr
	}
	req.SetInvocContext(ctx)
	return req, cmd, err
}