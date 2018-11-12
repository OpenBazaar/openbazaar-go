package cmds

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"testing"
)

func TestChanResponsePair(t *testing.T) {
	type testcase struct {
		values   []interface{}
		closeErr error
	}

	mkTest := func(tc testcase) func(*testing.T) {
		return func(t *testing.T) {
			cmd := &Command{}
			req, err := NewRequest(context.TODO(), nil, nil, nil, nil, cmd)
			if err != nil {
				t.Fatal("error building request", err)
			}
			re, res := NewChanResponsePair(req)

			var wg sync.WaitGroup

			wg.Add(1)
			go func() {
				for _, v := range tc.values {
					v_, err := res.Next()
					if err != nil {
						t.Error("Next returned unexpected error:", err)
					}
					if v != v_ {
						t.Errorf("Next returned unexpected value %q, expected %q", v_, v)
					}
				}

				_, err := res.Next()
				if tc.closeErr == nil || tc.closeErr == io.EOF {
					if err == nil {
						t.Error("Next returned nil error, expecting io.EOF")
					} else if err != io.EOF {
						t.Errorf("Next returned error %q, expecting io.EOF", err)
					}
				} else {
					if err != tc.closeErr {
						t.Errorf("Next returned error %q, expecting %q", err, tc.closeErr)
					}
				}

				wg.Done()
			}()

			for _, v := range tc.values {
				err := re.Emit(v)
				if err != nil {
					t.Error("Emit returned unexpected error:", err)
				}
			}

			re.CloseWithError(tc.closeErr)

			wg.Wait()
		}
	}

	tcs := []testcase{
		{values: []interface{}{1, 2, 3}},
		{values: []interface{}{1, 2, 3}, closeErr: io.EOF},
		{values: []interface{}{1, 2, 3}, closeErr: errors.New("an error occured")},
	}

	for i, tc := range tcs {
		t.Run(fmt.Sprint(i), mkTest(tc))
	}
}

func TestSingle1(t *testing.T) {
	cmd := &Command{}
	req, err := NewRequest(context.TODO(), nil, nil, nil, nil, cmd)
	if err != nil {
		t.Fatal("error building request", err)
	}
	re, res := NewChanResponsePair(req)

	wait := make(chan struct{})

	go func() {
		re.Emit(Single{42})

		err := re.Close()
		if err != ErrClosingClosedEmitter {
			t.Fatalf("expected double close error, got %v", err)
		}
		close(wait)
	}()

	v, err := res.Next()
	if err != nil {
		t.Fatal(err)
	}

	if v != 42 {
		t.Fatal("expected 42, got", v)
	}

	_, err = res.Next()
	if err != io.EOF {
		t.Fatal("expected EOF, got", err)
	}

	<-wait
}

func TestSingle2(t *testing.T) {
	cmd := &Command{}
	req, err := NewRequest(context.TODO(), nil, nil, nil, nil, cmd)
	if err != nil {
		t.Fatal("error building request", err)
	}
	re, res := NewChanResponsePair(req)

	re.Close()
	go func() {
		err := re.Emit(Single{42})
		if err != ErrClosedEmitter {
			t.Fatal("expected closed emitter error, got", err)
		}
	}()

	_, err = res.Next()
	if err != io.EOF {
		t.Fatal("expected EOF, got", err)
	}
}

func TestDoubleClose(t *testing.T) {
	cmd := &Command{}
	req, err := NewRequest(context.TODO(), nil, nil, nil, nil, cmd)
	if err != nil {
		t.Fatal("error building request", err)
	}
	re, _ := NewChanResponsePair(req)

	err = re.Close()
	if err != nil {
		t.Fatal("unexpected error closing re:", err)
	}

	err = re.Close()
	if err != ErrClosingClosedEmitter {
		t.Fatal("expected closed emitter error, got", err)
	}
}
