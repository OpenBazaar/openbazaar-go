package commands

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"unicode"

	"github.com/ipfs/go-ipfs/core/commands/e"

	cid "gx/ipfs/QmPSQnBKM9g7BaUcZCvswUJVscQ1ipjmwxN5PXCjkp9EQ7/go-cid"
	mhash "gx/ipfs/QmPnFwZ2JXKnXgMw8CdBPxn7FWh6LLdjUjxV1fKHuJnkr8/go-multihash"
	cidutil "gx/ipfs/QmQJSeE3CX4zos9qeaG8EhecEK9zvrTEfTG84J8C5NVRwt/go-cidutil"
	cmds "gx/ipfs/QmSXUokcP4TJpFfqozT69AVAYRtzXVMUjzQVkYX41R9Svs/go-ipfs-cmds"
	verifcid "gx/ipfs/QmVkMRSkXrpjqrroEXWuYBvDBnXCdMMY6gsKicBGVGUqKT/go-verifcid"
	cmdkit "gx/ipfs/Qmde5VP1qUkyQXKCfmEUA7bP64V2HAptbJ7phuPp7jXWwg/go-ipfs-cmdkit"
	mbase "gx/ipfs/QmekxXDhCxCJRNuzmHreuaT3BsuJcsjcXWNrtV9C8DRHtd/go-multibase"
)

var CidCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Convert and discover properties of CIDs",
	},
	Subcommands: map[string]*cmds.Command{
		"format": cidFmtCmd,
		"base32": base32Cmd,
		"bases":  basesCmd,
		"codecs": codecsCmd,
		"hashes": hashesCmd,
	},
}

const (
	cidFormatOptionName    = "f"
	cidVerisonOptionName   = "v"
	cidMultibaseOptionName = "b"
)

var cidFmtCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Format and convert a CID in various useful ways.",
		LongDescription: `
Format and converts <cid>'s in various useful ways.

The optional format string is a printf style format string:
` + cidutil.FormatRef,
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("cid", true, true, "Cids to format.").EnableStdin(),
	},
	Options: []cmdkit.Option{
		cmdkit.StringOption(cidFormatOptionName, "Printf style format string.").WithDefault("%s"),
		cmdkit.StringOption(cidVerisonOptionName, "CID version to convert to."),
		cmdkit.StringOption(cidMultibaseOptionName, "Multibase to display CID in."),
	},
	Run: func(req *cmds.Request, resp cmds.ResponseEmitter, env cmds.Environment) error {
		fmtStr, _ := req.Options[cidFormatOptionName].(string)
		verStr, _ := req.Options[cidVerisonOptionName].(string)
		baseStr, _ := req.Options[cidMultibaseOptionName].(string)

		opts := cidFormatOpts{}

		if strings.IndexByte(fmtStr, '%') == -1 {
			return fmt.Errorf("invalid format string: %s", fmtStr)
		}
		opts.fmtStr = fmtStr

		switch verStr {
		case "":
			// noop
		case "0":
			opts.verConv = toCidV0
		case "1":
			opts.verConv = toCidV1
		default:
			return fmt.Errorf("invalid cid version: %s", verStr)
		}

		if baseStr != "" {
			encoder, err := mbase.EncoderByName(baseStr)
			if err != nil {
				return err
			}
			opts.newBase = encoder.Encoding()
		} else {
			opts.newBase = mbase.Encoding(-1)
		}

		return emitCids(req, resp, opts)
	},
	PostRun: cmds.PostRunMap{
		cmds.CLI: streamResult(func(v interface{}, out io.Writer) nonFatalError {
			r := v.(*CidFormatRes)
			if r.ErrorMsg != "" {
				return nonFatalError(fmt.Sprintf("%s: %s", r.CidStr, r.ErrorMsg))
			}
			fmt.Fprintf(out, "%s\n", r.Formatted)
			return ""
		}),
	},
	Type: CidFormatRes{},
}

type CidFormatRes struct {
	CidStr    string // Original Cid String passed in
	Formatted string // Formated Result
	ErrorMsg  string // Error
}

var base32Cmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Convert CIDs to Base32 CID version 1.",
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("cid", true, true, "Cids to convert.").EnableStdin(),
	},
	Run: func(req *cmds.Request, resp cmds.ResponseEmitter, env cmds.Environment) error {
		opts := cidFormatOpts{
			fmtStr:  "%s",
			newBase: mbase.Encoding(mbase.Base32),
			verConv: toCidV1,
		}
		return emitCids(req, resp, opts)
	},
	PostRun: cidFmtCmd.PostRun,
	Type:    cidFmtCmd.Type,
}

type cidFormatOpts struct {
	fmtStr  string
	newBase mbase.Encoding
	verConv func(cid cid.Cid) (cid.Cid, error)
}

type argumentIterator struct {
	args []string
	body cmds.StdinArguments
}

func (i *argumentIterator) next() (string, bool) {
	if len(i.args) > 0 {
		arg := i.args[0]
		i.args = i.args[1:]
		return arg, true
	}
	if i.body == nil || !i.body.Scan() {
		return "", false
	}
	return strings.TrimSpace(i.body.Argument()), true
}

func (i *argumentIterator) err() error {
	if i.body == nil {
		return nil
	}
	return i.body.Err()
}

func emitCids(req *cmds.Request, resp cmds.ResponseEmitter, opts cidFormatOpts) error {
	itr := argumentIterator{req.Arguments, req.BodyArgs()}
	var emitErr error
	for emitErr == nil {
		cidStr, ok := itr.next()
		if !ok {
			break
		}
		res := &CidFormatRes{CidStr: cidStr}
		c, err := cid.Decode(cidStr)
		if err != nil {
			res.ErrorMsg = err.Error()
			emitErr = resp.Emit(res)
			continue
		}
		base := opts.newBase
		if base == -1 {
			base, _ = cid.ExtractEncoding(cidStr)
		}
		if opts.verConv != nil {
			c, err = opts.verConv(c)
			if err != nil {
				res.ErrorMsg = err.Error()
				emitErr = resp.Emit(res)
				continue
			}
		}
		str, err := cidutil.Format(opts.fmtStr, base, c)
		if _, ok := err.(cidutil.FormatStringError); ok {
			// no point in continuing if there is a problem with the format string
			return err
		}
		if err != nil {
			res.ErrorMsg = err.Error()
		} else {
			res.Formatted = str
		}
		emitErr = resp.Emit(res)
	}
	if emitErr != nil {
		return emitErr
	}
	err := itr.err()
	if err != nil {
		return err
	}
	return nil
}

func toCidV0(c cid.Cid) (cid.Cid, error) {
	if c.Type() != cid.DagProtobuf {
		return cid.Cid{}, fmt.Errorf("can't convert non-protobuf nodes to cidv0")
	}
	return cid.NewCidV0(c.Hash()), nil
}

func toCidV1(c cid.Cid) (cid.Cid, error) {
	return cid.NewCidV1(c.Type(), c.Hash()), nil
}

type CodeAndName struct {
	Code int
	Name string
}

const (
	prefixOptionName  = "prefix"
	numericOptionName = "numeric"
)

var basesCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "List available multibase encodings.",
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption(prefixOptionName, "also include the single leter prefixes in addition to the code"),
		cmdkit.BoolOption(numericOptionName, "also include numeric codes"),
	},
	Run: func(req *cmds.Request, resp cmds.ResponseEmitter, env cmds.Environment) error {
		var res []CodeAndName
		// use EncodingToStr in case at some point there are multiple names for a given code
		for code, name := range mbase.EncodingToStr {
			res = append(res, CodeAndName{int(code), name})
		}
		cmds.EmitOnce(resp, res)
		return nil
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeEncoder(func(req *cmds.Request, w io.Writer, val0 interface{}) error {
			prefixes, _ := req.Options[prefixOptionName].(bool)
			numeric, _ := req.Options[numericOptionName].(bool)
			val, ok := val0.([]CodeAndName)
			if !ok {
				return e.TypeErr(val, val0)
			}
			sort.Sort(multibaseSorter{val})
			for _, v := range val {
				code := v.Code
				if code < 32 || code >= 127 {
					// don't display non-printable prefixes
					code = ' '
				}
				switch {
				case prefixes && numeric:
					fmt.Fprintf(w, "%c %5d  %s\n", code, v.Code, v.Name)
				case prefixes:
					fmt.Fprintf(w, "%c  %s\n", code, v.Name)
				case numeric:
					fmt.Fprintf(w, "%5d  %s\n", v.Code, v.Name)
				default:
					fmt.Fprintf(w, "%s\n", v.Name)
				}
			}
			return nil
		}),
	},
	Type: []CodeAndName{},
}

const (
	codecsNumericOptionName = "numeric"
)

var codecsCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "List available CID codecs.",
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption(codecsNumericOptionName, "also include numeric codes"),
	},
	Run: func(req *cmds.Request, resp cmds.ResponseEmitter, env cmds.Environment) error {
		var res []CodeAndName
		// use CodecToStr as there are multiple names for a given code
		for code, name := range cid.CodecToStr {
			res = append(res, CodeAndName{int(code), name})
		}
		cmds.EmitOnce(resp, res)
		return nil
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeEncoder(func(req *cmds.Request, w io.Writer, val0 interface{}) error {
			numeric, _ := req.Options[codecsNumericOptionName].(bool)
			val, ok := val0.([]CodeAndName)
			if !ok {
				return e.TypeErr(val, val0)
			}
			sort.Sort(codeAndNameSorter{val})
			for _, v := range val {
				if numeric {
					fmt.Fprintf(w, "%5d  %s\n", v.Code, v.Name)
				} else {
					fmt.Fprintf(w, "%s\n", v.Name)
				}
			}
			return nil
		}),
	},
	Type: []CodeAndName{},
}

var hashesCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "List available multihashes.",
	},
	Options: codecsCmd.Options,
	Run: func(req *cmds.Request, resp cmds.ResponseEmitter, env cmds.Environment) error {
		var res []CodeAndName
		// use mhash.Codes in case at some point there are multiple names for a given code
		for code, name := range mhash.Codes {
			if !verifcid.IsGoodHash(code) {
				continue
			}
			res = append(res, CodeAndName{int(code), name})
		}
		cmds.EmitOnce(resp, res)
		return nil
	},
	Encoders: codecsCmd.Encoders,
	Type:     codecsCmd.Type,
}

type multibaseSorter struct {
	data []CodeAndName
}

func (s multibaseSorter) Len() int      { return len(s.data) }
func (s multibaseSorter) Swap(i, j int) { s.data[i], s.data[j] = s.data[j], s.data[i] }

func (s multibaseSorter) Less(i, j int) bool {
	a := unicode.ToLower(rune(s.data[i].Code))
	b := unicode.ToLower(rune(s.data[j].Code))
	if a != b {
		return a < b
	}
	// lowecase letters should come before uppercase
	return s.data[i].Code > s.data[j].Code
}

type codeAndNameSorter struct {
	data []CodeAndName
}

func (s codeAndNameSorter) Len() int           { return len(s.data) }
func (s codeAndNameSorter) Swap(i, j int)      { s.data[i], s.data[j] = s.data[j], s.data[i] }
func (s codeAndNameSorter) Less(i, j int) bool { return s.data[i].Code < s.data[j].Code }
