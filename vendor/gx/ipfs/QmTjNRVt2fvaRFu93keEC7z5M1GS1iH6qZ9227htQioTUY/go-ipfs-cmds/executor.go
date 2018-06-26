package cmds

import (
	"context"

	"gx/ipfs/QmceUdzxkimdYsgtX733uNgzf1DLHyBKN6ehGSp85ayppM/go-ipfs-cmdkit"
)

type Executor interface {
	Execute(req *Request, re ResponseEmitter, env Environment) error
}

// Environment is the environment passed to commands. The only required method
// is Context.
type Environment interface {
	// Context returns the environment's context.
	Context() context.Context
}

// MakeEnvironment takes a context and the request to construct the environment
// that is passed to the command's Run function.
// The user can define a function like this to pass it to cli.Run.
type MakeEnvironment func(context.Context, *Request) (Environment, error)

// MakeExecutor takes the request and environment variable to construct the
// executor that determines how to call the command - i.e. by calling Run or
// making an API request to a daemon.
// The user can define a function like this to pass it to cli.Run.
type MakeExecutor func(*Request, interface{}) (Executor, error)

func NewExecutor(root *Command) Executor {
	return &executor{
		root: root,
	}
}

type executor struct {
	root *Command
}

func (x *executor) Execute(req *Request, re ResponseEmitter, env Environment) (err error) {
	cmd := req.Command

	if cmd.Run == nil {
		return ErrNotCallable
	}

	err = cmd.CheckArguments(req)
	if err != nil {
		return err
	}

	// If this ResponseEmitter encodes messages (e.g. http, cli or writer - but not chan),
	// we need to update the encoding to the one specified by the command.
	if ee, ok := re.(EncodingEmitter); ok {
		encType := GetEncoding(req)

		// use JSON if text was requested but the command doesn't have a text-encoder
		if _, ok := cmd.Encoders[encType]; encType == Text && !ok {
			encType = JSON
		}

		if enc, ok := cmd.Encoders[encType]; ok {
			ee.SetEncoder(enc(req))
		} else if enc, ok := Encoders[encType]; ok {
			ee.SetEncoder(enc(req))
		} else {
			log.Errorf("unknown encoding %q, using json", encType)
			ee.SetEncoder(Encoders[JSON](req))
		}
	}

	if cmd.PreRun != nil {
		err = cmd.PreRun(req, env)
		if err != nil {
			return err
		}
	}

	if cmd.PostRun != nil {
		if typer, ok := re.(interface {
			Type() PostRunType
		}); ok && cmd.PostRun[typer.Type()] != nil {
			re = cmd.PostRun[typer.Type()](req, re)
		}
	}

	defer func() {
		re.Close()
	}()
	defer func() {
		// catch panics in Run (esp. from re.SetError)
		if v := recover(); v != nil {
			// if they are errors
			if e, ok := v.(error); ok {
				// use them as return error
				err = re.Emit(cmdkit.Error{Message: e.Error(), Code: cmdkit.ErrNormal})
				if err != nil {
					log.Errorf("recovered from command error %q but failed emitting it: %q", e, err)
				}
			} else {
				// otherwise keep panicking.
				panic(v)
			}
		}

	}()
	cmd.Run(req, re, env)
	return nil
}
