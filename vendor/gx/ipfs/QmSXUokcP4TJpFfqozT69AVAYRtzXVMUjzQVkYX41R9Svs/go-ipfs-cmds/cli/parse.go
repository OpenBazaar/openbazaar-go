package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"gx/ipfs/QmSXUokcP4TJpFfqozT69AVAYRtzXVMUjzQVkYX41R9Svs/go-ipfs-cmds"

	osh "gx/ipfs/QmXuBJ7DR6k3rmUEKtvVMhwjmXDuJgXXPUt4LQXKBMsU93/go-os-helper"
	logging "gx/ipfs/QmZChCsSt8DctjceaL56Eibc29CVQq4dGKRXC5JRZ6Ppae/go-log"
	"gx/ipfs/QmZMWMvWMVKCbHetJ4RgndbuEF1io2UpUxwQwtNjtYPzSC/go-ipfs-files"
	"gx/ipfs/Qmde5VP1qUkyQXKCfmEUA7bP64V2HAptbJ7phuPp7jXWwg/go-ipfs-cmdkit"
)

var log = logging.Logger("cmds/cli")
var msgStdinInfo = "ipfs: Reading from %s; send Ctrl-d to stop."

func init() {
	if osh.IsWindows() {
		msgStdinInfo = "ipfs: Reading from %s; send Ctrl-z to stop."
	}
}

// Parse parses the input commandline string (cmd, flags, and args).
// returns the corresponding command Request object.
//
// This function never returns nil, even on error.
func Parse(ctx context.Context, input []string, stdin *os.File, root *cmds.Command) (*cmds.Request, error) {
	req := &cmds.Request{Context: ctx}

	if err := parse(req, input, root); err != nil {
		return req, err
	}

	if err := req.FillDefaults(); err != nil {
		return req, err
	}

	// This is an ugly hack to maintain our current CLI interface while fixing
	// other stdin usage bugs. Let this serve as a warning, be careful about the
	// choices you make, they will haunt you forever.
	if len(req.Path) == 2 && req.Path[0] == "bootstrap" {
		if (req.Path[1] == "add" && req.Options["default"] == true) ||
			(req.Path[1] == "rm" && req.Options["all"] == true) {
			stdin = nil
		}
	}

	if err := parseArgs(req, root, stdin); err != nil {
		return req, err
	}

	// if no encoding was specified by user, default to plaintext encoding
	// (if command doesn't support plaintext, use JSON instead)
	if enc := req.Options[cmds.EncLong]; enc == "" {
		if req.Command.Encoders != nil && req.Command.Encoders[cmds.Text] != nil {
			req.SetOption(cmds.EncLong, cmds.Text)
		} else {
			req.SetOption(cmds.EncLong, cmds.JSON)
		}
	}

	return req, nil
}

func isHidden(req *cmds.Request) bool {
	h, ok := req.Options["hidden"].(bool)
	return h && ok
}

func isRecursive(req *cmds.Request) bool {
	rec, ok := req.Options[cmds.RecLong].(bool)
	return rec && ok
}

type parseState struct {
	cmdline []string
	i       int
}

func (st *parseState) done() bool {
	return st.i >= len(st.cmdline)
}

func (st *parseState) peek() string {
	return st.cmdline[st.i]
}

func parse(req *cmds.Request, cmdline []string, root *cmds.Command) (err error) {
	var (
		path = make([]string, 0, len(cmdline))
		args = make([]string, 0, len(cmdline))
		opts = cmdkit.OptMap{}
		cmd  = root
	)

	st := &parseState{cmdline: cmdline}

	// get root options
	optDefs, err := root.GetOptions([]string{})
	if err != nil {
		return err
	}

L:
	// don't range so we can seek
	for !st.done() {
		param := st.peek()
		switch {
		case param == "--":
			// use the rest as positional arguments
			args = append(args, st.cmdline[st.i+1:]...)
			break L
		case strings.HasPrefix(param, "--"):
			// long option
			k, v, err := st.parseLongOpt(optDefs)
			if err != nil {
				return err
			}

			if _, exists := opts[k]; exists {
				return fmt.Errorf("multiple values for option %q", k)
			}

			k = optDefs[k].Name()
			opts[k] = v

		case strings.HasPrefix(param, "-") && param != "-":
			// short options
			kvs, err := st.parseShortOpts(optDefs)
			if err != nil {
				return err
			}

			for _, kv := range kvs {
				kv.Key = optDefs[kv.Key].Names()[0]

				if _, exists := opts[kv.Key]; exists {
					return fmt.Errorf("multiple values for option %q", kv.Key)
				}

				opts[kv.Key] = kv.Value
			}
		default:
			arg := param
			// arg is a sub-command or a positional argument
			sub := cmd.Subcommands[arg]
			if sub != nil {
				cmd = sub
				path = append(path, arg)
				optDefs, err = root.GetOptions(path)
				if err != nil {
					return err
				}

				// If we've come across an external binary call, pass all the remaining
				// arguments on to it
				if cmd.External {
					args = append(args, st.cmdline[st.i+1:]...)
					break L
				}
			} else {
				args = append(args, arg)
				if len(path) == 0 {
					// found a typo or early argument
					return printSuggestions(args, root)
				}
			}
		}

		st.i++
	}

	req.Root = root
	req.Command = cmd
	req.Path = path
	req.Arguments = args
	req.Options = opts

	return nil
}

func parseArgs(req *cmds.Request, root *cmds.Command, stdin *os.File) error {
	argDefs := req.Command.Arguments

	// count required argument definitions
	var numRequired int
	for _, argDef := range argDefs {
		if argDef.Required {
			numRequired++
		}
	}

	inputs := req.Arguments

	// count number of values provided by user.
	// if there is at least one ArgDef, we can safely trigger the inputs loop
	// below to parse stdin.
	numInputs := len(inputs)

	if len(argDefs) > 0 && argDefs[len(argDefs)-1].SupportsStdin && stdin != nil {
		numInputs += 1
	}

	// if we have more arg values provided than argument definitions,
	// and the last arg definition is not variadic (or there are no definitions), return an error
	notVariadic := len(argDefs) == 0 || !argDefs[len(argDefs)-1].Variadic
	if notVariadic && len(inputs) > len(argDefs) {
		return printSuggestions(inputs, root)
	}

	stringArgs := make([]string, 0, numInputs)
	fileArgs := make(map[string]files.File)

	// the index of the current argument definition
	iArgDef := 0

	// remaining number of required arguments
	remRequired := numRequired

	for iInput := 0; iInput < numInputs; iInput++ {
		// remaining number of passed arguments
		remInputs := numInputs - iInput

		argDef := getArgDef(iArgDef, argDefs)

		// skip optional argument definitions if there aren't sufficient remaining inputs
		for remInputs <= remRequired && !argDef.Required {
			iArgDef++
			argDef = getArgDef(iArgDef, argDefs)
		}
		if argDef.Required {
			remRequired--
		}

		fillingVariadic := iArgDef+1 > len(argDefs)
		switch argDef.Type {
		case cmdkit.ArgString:
			if len(inputs) > 0 {
				stringArgs, inputs = append(stringArgs, inputs[0]), inputs[1:]
			} else if stdin != nil && argDef.SupportsStdin && !fillingVariadic {
				if r, err := maybeWrapStdin(stdin, msgStdinInfo); err == nil {
					fileArgs[stdin.Name()] = files.NewReaderFile("stdin", "", r, nil)
					stdin = nil
				}
			}
		case cmdkit.ArgFile:
			if len(inputs) > 0 {
				// treat stringArg values as file paths
				fpath := inputs[0]
				inputs = inputs[1:]
				var file files.File
				if fpath == "-" {
					r, err := maybeWrapStdin(stdin, msgStdinInfo)
					if err != nil {
						return err
					}

					fpath = stdin.Name()
					file = files.NewReaderFile("", fpath, r, nil)
				} else {
					nf, err := appendFile(fpath, argDef, isRecursive(req), isHidden(req))
					if err != nil {
						return err
					}

					file = nf
				}

				fileArgs[fpath] = file
			} else if stdin != nil && argDef.SupportsStdin &&
				argDef.Required && !fillingVariadic {
				r, err := maybeWrapStdin(stdin, msgStdinInfo)
				if err != nil {
					return err
				}

				fpath := stdin.Name()
				fileArgs[fpath] = files.NewReaderFile("", fpath, r, nil)
			}
		}

		iArgDef++
	}

	if iArgDef == len(argDefs)-1 && stdin != nil &&
		req.Command.Arguments[iArgDef].SupportsStdin {
		// handle this one at runtime, pretend it's there
		iArgDef++
	}

	// check to make sure we didn't miss any required arguments
	if len(argDefs) > iArgDef {
		for _, argDef := range argDefs[iArgDef:] {
			if argDef.Required {
				return fmt.Errorf("argument %q is required", argDef.Name)
			}
		}
	}

	req.Arguments = stringArgs
	if len(fileArgs) > 0 {
		req.Files = files.NewSliceFile("", "", filesMapToSortedArr(fileArgs))
	}

	return nil
}

func splitkv(opt string) (k, v string, ok bool) {
	split := strings.SplitN(opt, "=", 2)
	if len(split) == 2 {
		return split[0], split[1], true
	} else {
		return opt, "", false
	}
}

func parseOpt(opt, value string, opts map[string]cmdkit.Option) (interface{}, error) {
	optDef, ok := opts[opt]
	if !ok {
		return nil, fmt.Errorf("unknown option %q", opt)
	}

	v, err := optDef.Parse(value)
	if err != nil {
		return nil, err
	}
	return v, nil
}

type kv struct {
	Key   string
	Value interface{}
}

func (st *parseState) parseShortOpts(optDefs map[string]cmdkit.Option) ([]kv, error) {
	k, vStr, ok := splitkv(st.cmdline[st.i][1:])
	kvs := make([]kv, 0, len(k))

	if ok {
		// split at = successful
		v, err := parseOpt(k, vStr, optDefs)
		if err != nil {
			return nil, err
		}

		kvs = append(kvs, kv{Key: k, Value: v})

	} else {
	LOOP:
		for j := 0; j < len(k); {
			flag := k[j : j+1]
			od, ok := optDefs[flag]

			switch {
			case !ok:
				return nil, fmt.Errorf("unknown option %q", k)

			case od.Type() == cmdkit.Bool:
				// single char flags for bools
				kvs = append(kvs, kv{
					Key:   flag,
					Value: true,
				})
				j++

			case j < len(k)-1:
				// single char flag for non-bools (use the rest of the flag as value)
				rest := k[j+1:]

				v, err := parseOpt(flag, rest, optDefs)
				if err != nil {
					return nil, err
				}

				kvs = append(kvs, kv{Key: flag, Value: v})
				break LOOP

			case st.i < len(st.cmdline)-1:
				// single char flag for non-bools (use the next word as value)
				st.i++
				v, err := parseOpt(flag, st.cmdline[st.i], optDefs)
				if err != nil {
					return nil, err
				}

				kvs = append(kvs, kv{Key: flag, Value: v})
				break LOOP

			default:
				return nil, fmt.Errorf("missing argument for option %q", k)
			}
		}
	}

	return kvs, nil
}

func (st *parseState) parseLongOpt(optDefs map[string]cmdkit.Option) (string, interface{}, error) {
	k, v, ok := splitkv(st.peek()[2:])
	if !ok {
		optDef, ok := optDefs[k]
		if !ok {
			return "", nil, fmt.Errorf("unknown option %q", k)
		}
		if optDef.Type() == cmdkit.Bool {
			return k, true, nil
		} else if st.i < len(st.cmdline)-1 {
			st.i++
			v = st.peek()
		} else {
			return "", nil, fmt.Errorf("missing argument for option %q", k)
		}
	}

	optval, err := parseOpt(k, v, optDefs)
	return k, optval, err
}
func filesMapToSortedArr(fs map[string]files.File) []files.File {
	var names []string
	for name, _ := range fs {
		names = append(names, name)
	}

	sort.Strings(names)

	var out []files.File
	for _, f := range names {
		out = append(out, fs[f])
	}

	return out
}

func getArgDef(i int, argDefs []cmdkit.Argument) *cmdkit.Argument {
	if i < len(argDefs) {
		// get the argument definition (usually just argDefs[i])
		return &argDefs[i]

	} else if len(argDefs) > 0 {
		// but if i > len(argDefs) we use the last argument definition)
		return &argDefs[len(argDefs)-1]
	}

	// only happens if there aren't any definitions
	return nil
}

const notRecursiveFmtStr = "'%s' is a directory, use the '-%s' flag to specify directories"
const dirNotSupportedFmtStr = "Invalid path '%s', argument '%s' does not support directories"

func appendFile(fpath string, argDef *cmdkit.Argument, recursive, hidden bool) (files.File, error) {
	fpath = filepath.ToSlash(filepath.Clean(fpath))
	if fpath == "." {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		cwd, err = filepath.EvalSymlinks(cwd)
		if err != nil {
			return nil, err
		}
		fpath = filepath.ToSlash(cwd)
	}

	stat, err := os.Lstat(fpath)
	if err != nil {
		return nil, err
	}

	if stat.IsDir() {
		if !argDef.Recursive {
			return nil, fmt.Errorf(dirNotSupportedFmtStr, fpath, argDef.Name)
		}
		if !recursive {
			return nil, fmt.Errorf(notRecursiveFmtStr, fpath, cmds.RecShort)
		}
	}

	return files.NewSerialFile(path.Base(fpath), fpath, hidden, stat)
}

// Inform the user if a file is waiting on input
func maybeWrapStdin(f *os.File, msg string) (io.ReadCloser, error) {
	isTty, err := isTty(f)
	if err != nil {
		return nil, err
	}

	if isTty {
		return newMessageReader(f, fmt.Sprintf(msg, f.Name())), nil
	}

	return f, nil
}

func isTty(f *os.File) (bool, error) {
	fInfo, err := f.Stat()
	if err != nil {
		log.Error(err)
		return false, err
	}

	return (fInfo.Mode() & os.ModeCharDevice) != 0, nil
}

type messageReader struct {
	r       io.ReadCloser
	done    bool
	message string
}

func newMessageReader(r io.ReadCloser, msg string) io.ReadCloser {
	return &messageReader{
		r:       r,
		message: msg,
	}
}

func (r *messageReader) Read(b []byte) (int, error) {
	if !r.done {
		fmt.Fprintln(os.Stderr, r.message)
		r.done = true
	}

	return r.r.Read(b)
}

func (r *messageReader) Close() error {
	return r.r.Close()
}
