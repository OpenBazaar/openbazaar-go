package cmds

import (
	"context"
	"runtime/debug"
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

	if cmd.PreRun != nil {
		err = cmd.PreRun(req, env)
		if err != nil {
			return err
		}
	}

	// contains the error returned by PostRun
	errCh := make(chan error, 1)

	if cmd.PostRun != nil {
		if typer, ok := re.(interface {
			Type() PostRunType
		}); ok && cmd.PostRun[typer.Type()] != nil {
			var (
				res   Response
				lower = re
			)

			re, res = NewChanResponsePair(req)

			go func() {
				var closeErr error

				defer close(errCh)

				err := cmd.PostRun[typer.Type()](res, lower)
				closeErr = lower.CloseWithError(err)
				if closeErr == ErrClosingClosedEmitter {
					// ignore double close errors
					closeErr = nil
				}
				errCh <- closeErr

				if closeErr != nil && err != nil {
					log.Errorf("error closing connection: %s", closeErr)
					log.Errorf("close caused by error: %s", err)
				}
			}()
		}
	} else {
		// not using this channel today
		close(errCh)
	}

	defer func() {
		// catch panics in Run (esp. from re.SetError)
		if v := recover(); v != nil {
			log.Errorf("panic in command handler at %s", debug.Stack())

			// if they are errors
			if err, ok := v.(error); ok {
				// use them as return error
				closeErr := re.CloseWithError(err)
				if closeErr == ErrClosingClosedEmitter {
					// ignore double close errors
					closeErr = nil
				} else if closeErr != nil {
					log.Errorf("error closing connection: %s", closeErr)
					if err != nil {
						log.Errorf("close caused by error: %s", err)
					}
				}
			} else {
				// otherwise keep panicking.
				panic(v)
			}

			// wait for PostRun to finish
			<-errCh
		}
	}()
	err = cmd.Run(req, re, env)
	err = re.CloseWithError(err)
	if err == ErrClosingClosedEmitter {
		// ignore double close errors
		return nil
	} else if err != nil {
		return err
	}

	// return error from the PostRun Close
	return <-errCh
}
