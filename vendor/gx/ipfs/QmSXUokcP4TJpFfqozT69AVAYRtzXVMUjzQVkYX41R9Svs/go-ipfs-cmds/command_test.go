package cmds

import (
	"context"
	"errors"
	"fmt"
	"io"
	"testing"
	"time"

	"gx/ipfs/Qmde5VP1qUkyQXKCfmEUA7bP64V2HAptbJ7phuPp7jXWwg/go-ipfs-cmdkit"
)

// NOTE: helpers nopCloser, testEmitter, noop and writeCloser are defined in helpers_test.go

// TestOptionValidation tests whether option type validation works
func TestOptionValidation(t *testing.T) {
	cmd := &Command{
		Options: []cmdkit.Option{
			cmdkit.IntOption("b", "beep", "enables beeper"),
			cmdkit.StringOption("B", "boop", "password for booper"),
		},
		Run: noop,
	}

	type testcase struct {
		opts            map[string]interface{}
		NewRequestError string
	}

	mkTest := func(tc testcase) func(*testing.T) {
		return func(t *testing.T) {
			re := newTestEmitter(t)
			req, err := NewRequest(context.Background(), nil, tc.opts, nil, nil, cmd)
			if tc.NewRequestError == "" {
				if err != nil {
					t.Errorf("unexpected error %q", err)
				}

				cmd.Call(req, re, nil)
			} else {
				if err == nil {
					t.Errorf("Should have failed with error %q", tc.NewRequestError)
				} else if err.Error() != tc.NewRequestError {
					t.Errorf("expected error %q, got %q", tc.NewRequestError, err)
				}
			}
		}
	}

	tcs := []testcase{
		{
			opts:            map[string]interface{}{"boop": true},
			NewRequestError: `Option "boop" should be type "string", but got type "bool"`,
		},
		{opts: map[string]interface{}{"beep": 5}},
		{opts: map[string]interface{}{"beep": 5, "boop": "test"}},
		{opts: map[string]interface{}{"b": 5, "B": "test"}},
		{opts: map[string]interface{}{"foo": 5}},
		{opts: map[string]interface{}{EncLong: "json"}},
		{opts: map[string]interface{}{"beep": "100"}},
		{
			opts:            map[string]interface{}{"beep": ":)"},
			NewRequestError: `Could not convert value ":)" to type "int" (for option "-beep")`,
		},
	}

	for i, tc := range tcs {
		t.Run(fmt.Sprint(i), mkTest(tc))
	}
}

func TestRegistration(t *testing.T) {
	cmdA := &Command{
		Options: []cmdkit.Option{
			cmdkit.IntOption("beep", "number of beeps"),
		},
		Run: noop,
	}

	cmdB := &Command{
		Options: []cmdkit.Option{
			cmdkit.IntOption("beep", "number of beeps"),
		},
		Run: noop,
		Subcommands: map[string]*Command{
			"a": cmdA,
		},
	}

	path := []string{"a"}
	_, err := cmdB.GetOptions(path)
	if err == nil {
		t.Error("Should have failed (option name collision)")
	}
}

func TestResolving(t *testing.T) {
	cmdC := &Command{}
	cmdB := &Command{
		Subcommands: map[string]*Command{
			"c": cmdC,
		},
	}
	cmdB2 := &Command{}
	cmdA := &Command{
		Subcommands: map[string]*Command{
			"b": cmdB,
			"B": cmdB2,
		},
	}
	cmd := &Command{
		Subcommands: map[string]*Command{
			"a": cmdA,
		},
	}

	cmds, err := cmd.Resolve([]string{"a", "b", "c"})
	if err != nil {
		t.Error(err)
	}
	if len(cmds) != 4 || cmds[0] != cmd || cmds[1] != cmdA || cmds[2] != cmdB || cmds[3] != cmdC {
		t.Error("Returned command path is different than expected", cmds)
	}
}

func TestWalking(t *testing.T) {
	cmdA := &Command{
		Subcommands: map[string]*Command{
			"b": &Command{},
			"B": &Command{},
		},
	}
	i := 0
	cmdA.Walk(func(c *Command) {
		i = i + 1
	})
	if i != 3 {
		t.Error("Command tree walk didn't work, expected 3 got:", i)
	}
}

func TestHelpProcessing(t *testing.T) {
	cmdB := &Command{
		Helptext: cmdkit.HelpText{
			ShortDescription: "This is other short",
		},
	}
	cmdA := &Command{
		Helptext: cmdkit.HelpText{
			ShortDescription: "This is short",
		},
		Subcommands: map[string]*Command{
			"a": cmdB,
		},
	}
	cmdA.ProcessHelp()
	if len(cmdA.Helptext.LongDescription) == 0 {
		t.Error("LongDescription was not set on basis of ShortDescription")
	}
	if len(cmdB.Helptext.LongDescription) == 0 {
		t.Error("LongDescription was not set on basis of ShortDescription")
	}
}

type postRunTestCase struct {
	length      uint64
	err         *cmdkit.Error
	emit        []interface{}
	postRun     func(Response, ResponseEmitter) error
	next        []interface{}
	finalLength uint64
}

// TestPostRun tests whether commands with PostRun return the intended result
func TestPostRun(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var testcases = []postRunTestCase{
		postRunTestCase{
			length:      3,
			err:         nil,
			emit:        []interface{}{7},
			finalLength: 4,
			next:        []interface{}{14},
			postRun: func(res Response, re ResponseEmitter) error {
				l := res.Length()
				re.SetLength(l + 1)

				for {
					v, err := res.Next()
					t.Log("PostRun: Next returned", v, err)
					if err != nil {
						return err
					}

					i := v.(int)

					err = re.Emit(2 * i)
					if err != nil {
						return err
					}
				}
			},
		},
	}

	for _, tc := range testcases {
		cmd := &Command{
			Run: func(req *Request, re ResponseEmitter, env Environment) error {
				re.SetLength(tc.length)

				for _, v := range tc.emit {
					err := re.Emit(v)
					if err != nil {
						t.Fatal(err)
					}
				}
				return nil
			},
			PostRun: PostRunMap{
				CLI: tc.postRun,
			},
		}

		req, err := NewRequest(ctx, nil, map[string]interface{}{
			EncLong: CLI,
		}, nil, nil, cmd)
		if err != nil {
			t.Fatal(err)
		}

		opts := req.Options
		if opts == nil {
			t.Fatal("req.Options() is nil")
		}

		encTypeIface := opts[EncLong]
		if encTypeIface == nil {
			t.Fatal("req.Options()[EncLong] is nil")
		}

		encType := EncodingType(encTypeIface.(string))
		if encType == "" {
			t.Fatal("no encoding type")
		}

		if encType != CLI {
			t.Fatal("wrong encoding type")
		}

		postre, res := NewChanResponsePair(req)
		re, postres := NewChanResponsePair(req)

		go func() {
			err := cmd.PostRun[PostRunType(encType)](postres, postre)
			err = postre.CloseWithError(err)
			if err != nil {
				t.Error("error closing after PostRun: ", err)
			}
		}()

		cmd.Call(req, re, nil)

		l := res.Length()
		if l != tc.finalLength {
			t.Fatal("wrong final length")
		}

		for _, x := range tc.next {
			ch := make(chan interface{})

			go func() {
				v, err := res.Next()
				t.Log("next returned", v, err)
				if err != nil {
					close(ch)
					t.Fatal(err)
				}

				ch <- v
			}()

			select {
			case v, ok := <-ch:
				if !ok {
					t.Fatal("error checking all next values - channel closed")
				}
				if x != v {
					t.Fatalf("final check of emitted values failed. got %v but expected %v", v, x)
				}
			case <-time.After(50 * time.Millisecond):
				t.Fatal("too few values in next")
			}
		}

		_, err = res.Next()
		if err != io.EOF {
			t.Fatal("expected EOF, got", err)
		}
	}
}

func TestCancel(t *testing.T) {
	wait := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())

	req, err := NewRequest(ctx, nil, nil, nil, nil, &Command{})
	if err != nil {
		t.Fatal(err)
	}

	re, res := NewChanResponsePair(req)

	go func() {
		err := re.Emit("abc")
		if err != context.Canceled {
			t.Errorf("re:  expected context.Canceled but got %v", err)
		} else {
			t.Log("re.Emit err:", err)
		}
		re.Close()
		close(wait)
	}()

	cancel()

	_, err = res.Next()
	if err != context.Canceled {
		t.Errorf("res: expected context.Canceled but got %v", err)
	} else {
		t.Log("res.Emit err:", err)
	}
	<-wait
}

type testEmitterWithError struct{ errorCount int }

func (s *testEmitterWithError) Close() error {
	return nil
}

func (s *testEmitterWithError) SetLength(_ uint64) {}

func (s *testEmitterWithError) CloseWithError(err error) error {
	s.errorCount++
	return nil
}

func (s *testEmitterWithError) Emit(value interface{}) error {
	return nil
}

func TestEmitterExpectError(t *testing.T) {
	cmd := &Command{
		Run: func(req *Request, re ResponseEmitter, env Environment) error {
			return errors.New("an error occurred")
		},
	}

	re := &testEmitterWithError{}
	req, err := NewRequest(context.Background(), nil, nil, nil, nil, cmd)

	if err != nil {
		t.Error("Should have passed")
	}

	cmd.Call(req, re, nil)

	switch re.errorCount {
	case 0:
		t.Errorf("expected SetError to be called")
	case 1:
	default:
		t.Errorf("expected SetError to be called once, but was called %d times", re.errorCount)
	}
}
