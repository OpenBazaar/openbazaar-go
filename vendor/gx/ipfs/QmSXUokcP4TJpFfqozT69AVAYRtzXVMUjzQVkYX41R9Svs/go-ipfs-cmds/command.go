/*
Package commands provides an API for defining and parsing commands.

Supporting nested commands, options, arguments, etc.  The commands
package also supports a collection of marshallers for presenting
output to the user, including text, JSON, and XML marshallers.
*/

package cmds

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"gx/ipfs/Qmde5VP1qUkyQXKCfmEUA7bP64V2HAptbJ7phuPp7jXWwg/go-ipfs-cmdkit"

	logging "gx/ipfs/QmZChCsSt8DctjceaL56Eibc29CVQq4dGKRXC5JRZ6Ppae/go-log"
)

const DefaultOutputEncoding = JSON

var log = logging.Logger("cmds")

// Function is the type of function that Commands use.
// It reads from the Request, and writes results to the ResponseEmitter.
type Function func(*Request, ResponseEmitter, Environment) error

// PostRunMap is the map used in Command.PostRun.
type PostRunMap map[PostRunType]func(Response, ResponseEmitter) error

// Command is a runnable command, with input arguments and options (flags).
// It can also have Subcommands, to group units of work into sets.
type Command struct {
	Options   []cmdkit.Option
	Arguments []cmdkit.Argument
	PreRun    func(req *Request, env Environment) error

	// Run is the function that processes the request to generate a response.
	// Note that when executing the command over the HTTP API you can only read
	// after writing when using multipart requests. The request body will not be
	// available for reading after the HTTP connection has been written to.
	Run      Function
	PostRun  PostRunMap
	Encoders EncoderMap
	Helptext cmdkit.HelpText

	// External denotes that a command is actually an external binary.
	// fewer checks and validations will be performed on such commands.
	External bool

	// Type describes the type of the output of the Command's Run Function.
	// In precise terms, the value of Type is an instance of the return type of
	// the Run Function.
	//
	// ie. If command Run returns &Block{}, then Command.Type == &Block{}
	Type        interface{}
	Subcommands map[string]*Command
}

var (
	// ErrNotCallable signals a command that cannot be called.
	ErrNotCallable = ClientError("This command can't be called directly. Try one of its subcommands.")

	// ErrNoFormatter signals that the command can not be formatted.
	ErrNoFormatter = ClientError("This command cannot be formatted to plain text")

	// ErrIncorrectType signales that the commands returned a value with unexpected type.
	ErrIncorrectType = errors.New("The command returned a value with a different type than expected")
)

// Call invokes the command for the given Request
func (c *Command) Call(req *Request, re ResponseEmitter, env Environment) {
	var closeErr error

	err := c.call(req, re, env)
	if err != nil {
		log.Debugf("error occured in call, closing with error: %s", err)
	}

	closeErr = re.CloseWithError(err)
	// ignore double close errors
	if closeErr != nil && closeErr != ErrClosingClosedEmitter {
		log.Errorf("error closing ResponseEmitter: %s", closeErr)
	}
}

func (c *Command) call(req *Request, re ResponseEmitter, env Environment) error {
	cmd, err := c.Get(req.Path)
	if err != nil {
		log.Errorf("could not get cmd from path %q: %q", req.Path, err)
		return err
	}

	if cmd.Run == nil {
		log.Errorf("returned command has nil Run function")
		return err
	}

	err = cmd.CheckArguments(req)
	if err != nil {
		log.Errorf("CheckArguments returned an error for path %q: %q", req.Path, err)
		return err
	}

	return cmd.Run(req, re, env)
}

// Resolve returns the subcommands at the given path
func (c *Command) Resolve(pth []string) ([]*Command, error) {
	cmds := make([]*Command, len(pth)+1)
	cmds[0] = c

	cmd := c
	for i, name := range pth {
		cmd = cmd.Subcommands[name]

		if cmd == nil {
			pathS := strings.Join(pth[:i], "/")
			return nil, fmt.Errorf("undefined command: %q", pathS)
		}

		cmds[i+1] = cmd
	}

	return cmds, nil
}

// Get resolves and returns the Command addressed by path
func (c *Command) Get(path []string) (*Command, error) {
	cmds, err := c.Resolve(path)
	if err != nil {
		return nil, err
	}
	return cmds[len(cmds)-1], nil
}

// GetOptions returns the options in the given path of commands
func (c *Command) GetOptions(path []string) (map[string]cmdkit.Option, error) {
	options := make([]cmdkit.Option, 0, len(c.Options))

	cmds, err := c.Resolve(path)
	if err != nil {
		return nil, err
	}

	for _, cmd := range cmds {
		options = append(options, cmd.Options...)
	}

	optionsMap := make(map[string]cmdkit.Option)
	for _, opt := range options {
		for _, name := range opt.Names() {
			if _, found := optionsMap[name]; found {
				return nil, fmt.Errorf("option name %q used multiple times", name)
			}

			optionsMap[name] = opt
		}
	}

	return optionsMap, nil
}

// DebugValidate checks if the command tree is well-formed.
//
// This operation is slow and should be called from tests only.
func (c *Command) DebugValidate() map[string][]error {
	errs := make(map[string][]error)
	var visit func(path string, cm *Command)

	liveOptions := make(map[string]struct{})
	visit = func(path string, cm *Command) {
		expectOptional := false
		for i, argDef := range cm.Arguments {
			// No required arguments after optional arguments.
			if argDef.Required {
				if expectOptional {
					errs[path] = append(errs[path], fmt.Errorf("required argument %s after optional arguments", argDef.Name))
					return
				}
			} else {
				expectOptional = true
			}

			// variadic arguments and those supporting stdin must be last
			if (argDef.Variadic || argDef.SupportsStdin) && i != len(cm.Arguments)-1 {
				errs[path] = append(errs[path], fmt.Errorf("variadic and/or optional argument %s must be last", argDef.Name))
			}
		}

		var goodOptions []string
		for _, option := range cm.Options {
			for _, name := range option.Names() {
				if _, ok := liveOptions[name]; ok {
					errs[path] = append(errs[path], fmt.Errorf("duplicate option name %s", name))
				} else {
					goodOptions = append(goodOptions, name)
					liveOptions[name] = struct{}{}
				}
			}
		}
		for scName, sc := range cm.Subcommands {
			visit(fmt.Sprintf("%s/%s", path, scName), sc)
		}

		for _, name := range goodOptions {
			delete(liveOptions, name)
		}
	}
	visit("", c)
	if len(errs) == 0 {
		errs = nil
	}
	return errs
}

// CheckArguments checks that we have all the required string arguments, loading
// any from stdin if necessary.
func (c *Command) CheckArguments(req *Request) error {
	if len(c.Arguments) == 0 {
		return nil
	}

	lastArg := c.Arguments[len(c.Arguments)-1]
	if req.bodyArgs == nil && // check this as we can end up calling CheckArguments multiple times. See #80.
		lastArg.SupportsStdin &&
		lastArg.Type == cmdkit.ArgString &&
		req.Files != nil {

		fi, err := req.Files.NextFile()
		switch err {
		case io.EOF:
		case nil:
			req.bodyArgs = newArguments(fi)
			// Can't pass files and stdin arguments.
			req.Files = nil
		default:
			// io error.
			return err
		}
	}

	// iterate over the arg definitions
	requiredStringArgs := 0 // number of required string arguments
	for _, argDef := range req.Command.Arguments {
		// Is this a string?
		if argDef.Type != cmdkit.ArgString {
			// No, skip it.
			continue
		}

		// No more required arguments?
		if !argDef.Required {
			// Yes, we're all done.
			break
		}
		requiredStringArgs++

		// Do we have enough string arguments?
		if requiredStringArgs <= len(req.Arguments) {
			// all good
			continue
		}

		// Can we get it from stdin?
		if argDef.SupportsStdin && req.bodyArgs != nil {
			if req.bodyArgs.Scan() {
				// Found it!
				req.Arguments = append(req.Arguments, req.bodyArgs.Argument())
				continue
			}
			if err := req.bodyArgs.Err(); err != nil {
				return err
			}
			// No, just missing.
		}
		return fmt.Errorf("argument %q is required", argDef.Name)
	}

	return nil
}

type CommandVisitor func(*Command)

// Walks tree of all subcommands (including this one)
func (c *Command) Walk(visitor CommandVisitor) {
	visitor(c)
	for _, sub := range c.Subcommands {
		sub.Walk(visitor)
	}
}

func (c *Command) ProcessHelp() {
	c.Walk(func(cm *Command) {
		ht := &cm.Helptext
		if len(ht.LongDescription) == 0 {
			ht.LongDescription = ht.ShortDescription
		}
	})
}

func ClientError(msg string) error {
	return &cmdkit.Error{Code: cmdkit.ErrClient, Message: msg}
}
