package goprocessctx

import (
	"context"
	"testing"
	"time"

	"github.com/jbenet/goprocess"
)

func testClosing(t *testing.T, p goprocess.Process, cancel context.CancelFunc) {
	select {
	case <-p.Closing():
		t.Fatal("closed")
	case <-p.Closed():
		t.Fatal("closed")
	case <-time.After(time.Second):
	}

	cancel()

	select {
	case <-p.Closed():
	case <-time.After(time.Second):
		t.Fatal("should have closed")
	}

}

func TestWithContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	testClosing(t, WithContext(ctx), cancel)
}

func TestWithAndTeardown(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := false
	p := WithContextAndTeardown(ctx, func() error {
		done = true
		return nil
	})

	select {
	case <-p.Closing():
		t.Fatal("closed")
	case <-p.Closed():
		t.Fatal("closed")
	case <-time.After(time.Second):
	}

	if done {
		t.Fatal("closed early")
	}

	cancel()

	select {
	case <-p.Closed():
	case <-time.After(time.Second):
		t.Fatal("should have closed")
	}

	if !done {
		t.Fatal("failed to close")
	}
}

func TestWaitForContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	p := goprocess.WithTeardown(func() error {
		close(done)
		return nil
	})

	WaitForContext(ctx, p)

	go func() {
		p.Close()
	}()

	select {
	case <-p.Closing():
	case <-time.After(time.Second):
		t.Fatal("should have started closing")
	}

	select {
	case <-p.Closed():
		t.Fatal("should not have closed")
	case <-time.After(time.Second):
	}

	cancel()

	select {
	case <-p.Closed():
	case <-time.After(time.Second):
		t.Fatal("should have closed")
	}
}

func TestCloseAfterContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	p := goprocess.WithTeardown(func() error {
		return nil
	})

	CloseAfterContext(p, ctx)
	testClosing(t, p, cancel)
}

func TestWithProcessClosing(t *testing.T) {
	t.Parallel()

	ctx := context.WithValue(context.Background(), "foo", "bar")

	p := goprocess.WithTeardown(func() error { return nil })

	ctx = WithProcessClosing(ctx, p)
	if ctx.Value("foo") != "bar" {
		t.Fatal("context value not preserved")
	}

	select {
	case <-ctx.Done():
		t.Fatal("should not have been canceled")
	case <-time.After(time.Second):
	}

	p.Close()

	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("should have been canceled")
	}
}

func TestWithProcessClosed(t *testing.T) {
	t.Parallel()

	ctx := context.WithValue(context.Background(), "foo", "bar")

	closeBlock := make(chan struct{})
	p := goprocess.WithTeardown(func() error { <-closeBlock; return nil })

	ctx = WithProcessClosed(ctx, p)
	if ctx.Value("foo") != "bar" {
		t.Fatal("context value not preserved")
	}

	select {
	case <-ctx.Done():
		t.Fatal("should not have been canceled")
	case <-time.After(time.Second):
	}

	closeWait := make(chan struct{})
	go func() {
		defer close(closeWait)
		p.Close()
	}()

	select {
	case <-ctx.Done():
		t.Fatal("should not have been canceled")
	case <-time.After(time.Second):
	}

	close(closeBlock)
	<-closeWait

	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("should have been canceled")
	}
}
