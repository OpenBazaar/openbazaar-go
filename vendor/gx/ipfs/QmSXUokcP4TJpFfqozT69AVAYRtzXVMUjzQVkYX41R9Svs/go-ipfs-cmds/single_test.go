package cmds

import (
	"context"
	"io"
	"sync"
	"testing"
)

func TestSingleChan(t *testing.T) {
	req, err := NewRequest(context.Background(), nil, nil, nil, nil, &Command{})
	if err != nil {
		t.Fatal(err)
	}

	re, res := NewChanResponsePair(req)

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()

		if err := EmitOnce(re, "test"); err != nil {
			t.Fatal(err)
		}

		err := re.Emit("test")
		if err != ErrClosedEmitter {
			t.Errorf("expected emit error %q, got: %v", ErrClosedEmitter, err)
		}

		err = re.Close()
		if err != ErrClosingClosedEmitter {
			t.Errorf("expected close error %q, got: %v", ErrClosingClosedEmitter, err)
		}
	}()

	v, err := res.Next()
	if err != nil {
		t.Fatal(err)
	}

	if str, ok := v.(string); !ok || str != "test" {
		t.Fatalf("expected %#v, got %#v", "foo", str)
	}

	if _, err = res.Next(); err != io.EOF {
		t.Fatalf("expected %#v, got %#v", io.EOF, err)
	}

	wg.Wait()
}

func TestSingleWriter(t *testing.T) {
	req, err := NewRequest(context.Background(), nil, nil, nil, nil, &Command{})
	if err != nil {
		t.Fatal(err)
	}

	pr, pw := io.Pipe()
	re, err := NewWriterResponseEmitter(pw, req)
	if err != nil {
		t.Fatal(err)
	}
	res, err := NewReaderResponse(pr, req)
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		if err := EmitOnce(re, "test"); err != nil {
			t.Fatal(err)
		}

		err := re.Emit("this should not be sent")
		if err != ErrClosedEmitter {
			t.Errorf("expected emit error %q, got: %v", ErrClosedEmitter, err)
		}

		err = re.Close()
		if err != ErrClosingClosedEmitter {
			t.Errorf("expected close error %q, got: %v", ErrClosingClosedEmitter, err)
		}
		wg.Done()
	}()

	v, err := res.Next()
	if err != nil {
		t.Fatal(err)
	}

	if str, ok := v.(string); !ok || str != "test" {
		t.Fatalf("expected %#v, got %#v", "foo", str)
	}

	if v, err = res.Next(); err != io.EOF {
		t.Log(v, err)
		t.Fatalf("expected %#v, got %#v", io.EOF, err)
	}

	wg.Wait()
}
