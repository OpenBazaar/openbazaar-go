package adder

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"gx/ipfs/QmSXUokcP4TJpFfqozT69AVAYRtzXVMUjzQVkYX41R9Svs/go-ipfs-cmds"
	"gx/ipfs/QmSXUokcP4TJpFfqozT69AVAYRtzXVMUjzQVkYX41R9Svs/go-ipfs-cmds/cli"
	"gx/ipfs/Qmde5VP1qUkyQXKCfmEUA7bP64V2HAptbJ7phuPp7jXWwg/go-ipfs-cmdkit"
)

// AddStatus describes the progress of the add operation
type AddStatus struct {
	// Current is the current value of the sum.
	Current int

	// Left is how many summands are left
	Left int
}

// Define the root of the commands
var RootCmd = &cmds.Command{
	Subcommands: map[string]*cmds.Command{
		// the simplest way to make an adder
		"simpleAdd": &cmds.Command{
			Arguments: []cmdkit.Argument{
				cmdkit.StringArg("summands", true, true, "values that are supposed to be summed"),
			},
			Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
				sum := 0

				for i, str := range req.Arguments {
					num, err := strconv.Atoi(str)
					if err != nil {
						return err
					}

					sum += num
					err = re.Emit(fmt.Sprintf("intermediate result: %d; %d left", sum, len(req.Arguments)-i-1))
					if err != nil {
						return err
					}
				}

				return re.Emit(fmt.Sprintf("total: %d", sum))
			},
		},
		// a bit more sophisticated
		"encodeAdd": &cmds.Command{
			Arguments: []cmdkit.Argument{
				cmdkit.StringArg("summands", true, true, "values that are supposed to be summed"),
			},
			Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
				sum := 0

				for i, str := range req.Arguments {
					num, err := strconv.Atoi(str)
					if err != nil {
						return err
					}

					sum += num
					err = re.Emit(&AddStatus{
						Current: sum,
						Left:    len(req.Arguments) - i - 1,
					})
					if err != nil {
						return err
					}

					time.Sleep(200 * time.Millisecond)
				}
				return nil
			},
			Type: &AddStatus{},
			Encoders: cmds.EncoderMap{
				// This defines how to encode these values as text. Other possible encodings are XML and JSON.
				cmds.Text: cmds.MakeEncoder(func(req *cmds.Request, w io.Writer, v interface{}) error {
					s, ok := v.(*AddStatus)
					if !ok {
						return fmt.Errorf("cast error, got type %T", v)
					}

					if s.Left == 0 {
						fmt.Fprintln(w, "total:", s.Current)
					} else {
						fmt.Fprintf(w, "intermediate result: %d; %d left\n", s.Current, s.Left)
					}

					return nil
				}),
			},
		},
		// the best UX
		"postRunAdd": &cmds.Command{
			Arguments: []cmdkit.Argument{
				cmdkit.StringArg("summands", true, true, "values that are supposed to be summed"),
			},
			// this is the same as for encoderAdd
			Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
				sum := 0

				for i, str := range req.Arguments {
					num, err := strconv.Atoi(str)
					if err != nil {
						return err
					}

					sum += num
					err = re.Emit(&AddStatus{
						Current: sum,
						Left:    len(req.Arguments) - i - 1,
					})
					if err != nil {
						return err
					}

					time.Sleep(200 * time.Millisecond)
				}
				return nil
			},
			Type: &AddStatus{},
			PostRun: cmds.PostRunMap{
				cmds.CLI: func(res cmds.Response, re cmds.ResponseEmitter) error {
					defer re.Close()
					defer fmt.Println()

					// length of line at last iteration
					var lastLen int

					for {
						v, err := res.Next()
						if err == io.EOF {
							return nil
						}
						if err != nil {
							return err
						}

						fmt.Print("\r" + strings.Repeat(" ", lastLen))

						s := v.(*AddStatus)
						if s.Left > 0 {
							lastLen, _ = fmt.Printf("\rcalculation sum... current: %d; left: %d", s.Current, s.Left)
						} else {
							lastLen, _ = fmt.Printf("\rsum is %d.", s.Current)
						}
					}
				},
			},
		},
		// how to set program's return value
		"exitAdd": &cmds.Command{
			Arguments: []cmdkit.Argument{
				cmdkit.StringArg("summands", true, true, "values that are supposed to be summed"),
			},
			// this is the same as for encoderAdd
			Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
				sum := 0

				for i, str := range req.Arguments {
					num, err := strconv.Atoi(str)
					if err != nil {
						return err
					}

					sum += num
					err = re.Emit(&AddStatus{
						Current: sum,
						Left:    len(req.Arguments) - i - 1,
					})
					if err != nil {
						return err
					}

					time.Sleep(200 * time.Millisecond)
				}
				return nil
			},
			Type: &AddStatus{},
			PostRun: cmds.PostRunMap{
				cmds.CLI: func(res cmds.Response, re cmds.ResponseEmitter) error {
					clire := re.(cli.ResponseEmitter)

					defer re.Close()
					defer fmt.Println()

					// length of line at last iteration
					var lastLen int

					var exit int
					defer func() {
						clire.Exit(exit)
					}()

					for {
						v, err := res.Next()
						if err == io.EOF {
							return nil
						}
						if err != nil {
							return err
						}

						fmt.Print("\r" + strings.Repeat(" ", lastLen))

						s := v.(*AddStatus)
						if s.Left > 0 {
							lastLen, _ = fmt.Printf("\rcalculation sum... current: %d; left: %d", s.Current, s.Left)
						} else {
							lastLen, _ = fmt.Printf("\rsum is %d.", s.Current)
							exit = s.Current
						}
					}
				},
			},
		},
	},
}
