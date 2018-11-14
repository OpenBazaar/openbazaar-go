package commands

import (
	"fmt"
	"io"
	"text/tabwriter"

	cmdenv "github.com/ipfs/go-ipfs/core/commands/cmdenv"
	"github.com/ipfs/go-ipfs/core/commands/e"
	"github.com/ipfs/go-ipfs/core/coreapi/interface/options"

	"gx/ipfs/QmSXUokcP4TJpFfqozT69AVAYRtzXVMUjzQVkYX41R9Svs/go-ipfs-cmds"
	"gx/ipfs/Qmde5VP1qUkyQXKCfmEUA7bP64V2HAptbJ7phuPp7jXWwg/go-ipfs-cmdkit"
)

var KeyCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Create and list IPNS name keypairs",
		ShortDescription: `
'ipfs key gen' generates a new keypair for usage with IPNS and 'ipfs name
publish'.

  > ipfs key gen --type=rsa --size=2048 mykey
  > ipfs name publish --key=mykey QmSomeHash

'ipfs key list' lists the available keys.

  > ipfs key list
  self
  mykey
		`,
	},
	Subcommands: map[string]*cmds.Command{
		"gen":    keyGenCmd,
		"list":   keyListCmd,
		"rename": keyRenameCmd,
		"rm":     keyRmCmd,
	},
}

type KeyOutput struct {
	Name string
	Id   string
}

type KeyOutputList struct {
	Keys []KeyOutput
}

// KeyRenameOutput define the output type of keyRenameCmd
type KeyRenameOutput struct {
	Was       string
	Now       string
	Id        string
	Overwrite bool
}

const (
	keyStoreTypeOptionName = "type"
	keyStoreSizeOptionName = "size"
)

var keyGenCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Create a new keypair",
	},
	Options: []cmdkit.Option{
		cmdkit.StringOption(keyStoreTypeOptionName, "t", "type of the key to create [rsa, ed25519]"),
		cmdkit.IntOption(keyStoreSizeOptionName, "s", "size of the key to generate"),
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("name", true, false, "name of key to create"),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env)
		if err != nil {
			return err
		}

		typ, f := req.Options[keyStoreTypeOptionName].(string)
		if !f {
			return fmt.Errorf("please specify a key type with --type")
		}

		name := req.Arguments[0]
		if name == "self" {
			return fmt.Errorf("cannot create key with name 'self'")
		}

		opts := []options.KeyGenerateOption{options.Key.Type(typ)}

		size, sizefound := req.Options[keyStoreSizeOptionName].(int)
		if sizefound {
			opts = append(opts, options.Key.Size(size))
		}

		key, err := api.Key().Generate(req.Context, name, opts...)

		if err != nil {
			return err
		}

		return cmds.EmitOnce(res, &KeyOutput{
			Name: name,
			Id:   key.ID().Pretty(),
		})
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeEncoder(func(req *cmds.Request, w io.Writer, v interface{}) error {
			k, ok := v.(*KeyOutput)
			if !ok {
				return e.TypeErr(k, v)
			}

			_, err := w.Write([]byte(k.Id + "\n"))
			return err
		}),
	},
	Type: KeyOutput{},
}

var keyListCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "List all local keypairs",
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption("l", "Show extra information about keys."),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env)
		if err != nil {
			return err
		}

		keys, err := api.Key().List(req.Context)
		if err != nil {
			return err
		}

		list := make([]KeyOutput, 0, len(keys))

		for _, key := range keys {
			list = append(list, KeyOutput{Name: key.Name(), Id: key.ID().Pretty()})
		}

		return cmds.EmitOnce(res, &KeyOutputList{list})
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: keyOutputListMarshaler(),
	},
	Type: KeyOutputList{},
}

const (
	keyStoreForceOptionName = "force"
)

var keyRenameCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Rename a keypair",
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("name", true, false, "name of key to rename"),
		cmdkit.StringArg("newName", true, false, "new name of the key"),
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption(keyStoreForceOptionName, "f", "Allow to overwrite an existing key."),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env)
		if err != nil {
			return err
		}

		name := req.Arguments[0]
		newName := req.Arguments[1]
		force, _ := req.Options[keyStoreForceOptionName].(bool)

		key, overwritten, err := api.Key().Rename(req.Context, name, newName, options.Key.Force(force))
		if err != nil {
			return err
		}

		return cmds.EmitOnce(res, &KeyRenameOutput{
			Was:       name,
			Now:       newName,
			Id:        key.ID().Pretty(),
			Overwrite: overwritten,
		})
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeEncoder(func(req *cmds.Request, w io.Writer, v interface{}) error {
			k, ok := v.(*KeyRenameOutput)
			if !ok {
				return fmt.Errorf("expected a KeyRenameOutput as command result")
			}

			if k.Overwrite {
				fmt.Fprintf(w, "Key %s renamed to %s with overwriting\n", k.Id, k.Now)
			} else {
				fmt.Fprintf(w, "Key %s renamed to %s\n", k.Id, k.Now)
			}
			return nil
		}),
	},
	Type: KeyRenameOutput{},
}

var keyRmCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Remove a keypair",
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("name", true, true, "names of keys to remove").EnableStdin(),
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption("l", "Show extra information about keys."),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env)
		if err != nil {
			return err
		}

		names := req.Arguments

		list := make([]KeyOutput, 0, len(names))
		for _, name := range names {
			key, err := api.Key().Remove(req.Context, name)
			if err != nil {
				return err
			}

			list = append(list, KeyOutput{Name: name, Id: key.ID().Pretty()})
		}

		return cmds.EmitOnce(res, &KeyOutputList{list})
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: keyOutputListMarshaler(),
	},
	Type: KeyOutputList{},
}

func keyOutputListMarshaler() cmds.EncoderFunc {
	return cmds.MakeEncoder(func(req *cmds.Request, w io.Writer, v interface{}) error {
		withID, _ := req.Options["l"].(bool)

		list, ok := v.(*KeyOutputList)
		if !ok {
			return e.TypeErr(list, v)
		}

		tw := tabwriter.NewWriter(w, 1, 2, 1, ' ', 0)
		for _, s := range list.Keys {
			if withID {
				fmt.Fprintf(tw, "%s\t%s\t\n", s.Id, s.Name)
			} else {
				fmt.Fprintf(tw, "%s\n", s.Name)
			}
		}
		tw.Flush()
		return nil
	})
}
