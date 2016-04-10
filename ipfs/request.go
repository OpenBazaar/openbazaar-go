package ipfs

import (
	cmds "github.com/ipfs/go-ipfs/commands"
	commands "github.com/ipfs/go-ipfs/core/commands"
	cli "github.com/ipfs/go-ipfs/commands/cli"
)


var Root = &cmds.Command{
	Options:  commands.Root.Options,
	Helptext: commands.Root.Helptext,
}

var commandsClientCmd = commands.CommandsCmd(Root)

var localCommands = map[string]*cmds.Command{
	"commands": commandsClientCmd,
}

func NewRequest(args []string) (cmds.Request, *cmds.Command, error) {
	Root.Subcommands = localCommands
	for k, v := range commands.Root.Subcommands {
		if _, found := Root.Subcommands[k]; !found {
			Root.Subcommands[k] = v
		}
	}
	req, cmd, _, err := cli.Parse(args, nil, Root)
	return req, cmd, err
}