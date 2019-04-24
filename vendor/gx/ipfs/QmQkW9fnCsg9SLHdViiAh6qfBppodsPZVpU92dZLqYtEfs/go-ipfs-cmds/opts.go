package cmds

import (
	"gx/ipfs/Qmde5VP1qUkyQXKCfmEUA7bP64V2HAptbJ7phuPp7jXWwg/go-ipfs-cmdkit"
)

// Flag names
const (
	EncShort     = "enc"
	EncLong      = "encoding"
	RecShort     = "r"
	RecLong      = "recursive"
	ChanOpt      = "stream-channels"
	TimeoutOpt   = "timeout"
	OptShortHelp = "h"
	OptLongHelp  = "help"
	DerefLong    = "dereference-args"
)

// options that are used by this package
var OptionEncodingType = cmdkit.StringOption(EncLong, EncShort, "The encoding type the output should be encoded with (json, xml, or text)").WithDefault("text")
var OptionRecursivePath = cmdkit.BoolOption(RecLong, RecShort, "Add directory paths recursively")
var OptionStreamChannels = cmdkit.BoolOption(ChanOpt, "Stream channel output")
var OptionTimeout = cmdkit.StringOption(TimeoutOpt, "Set a global timeout on the command")
var OptionDerefArgs = cmdkit.BoolOption(DerefLong, "Symlinks supplied in arguments are dereferenced")
