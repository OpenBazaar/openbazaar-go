package cli

import (
	"bytes"
	"context"
	"testing"

	"gx/ipfs/QmSXUokcP4TJpFfqozT69AVAYRtzXVMUjzQVkYX41R9Svs/go-ipfs-cmds"
)

func TestSingle(t *testing.T) {
	req, err := cmds.NewRequest(context.Background(), nil, nil, nil, nil, &cmds.Command{})
	if err != nil {
		t.Fatal(err)
	}

	var bufout, buferr bytes.Buffer

	re, exitCh, err := NewResponseEmitter(&bufout, &buferr, req)
	if err != nil {
		t.Fatal(err)
	}

	wait := make(chan struct{})

	go func() {
		if err := cmds.EmitOnce(re, "test"); err != nil {
			t.Fatal(err)
		}

		err := re.Emit("this should not be emitted")
		if err != cmds.ErrClosedEmitter {
			t.Errorf("expected emit error %q, got: %v", cmds.ErrClosedEmitter, err)
		}

		err = re.Close()
		if err != cmds.ErrClosingClosedEmitter {
			t.Errorf("expected close error %q, got: %v", cmds.ErrClosingClosedEmitter, err)
		}
		wait <- struct{}{}
	}()

	exitCode := <-exitCh
	if exitCode != 0 {
		t.Errorf("expected exit code 0, got: %v", exitCode)
	}

	str := bufout.String()
	if str != "test\n" {
		t.Fatalf("expected %#v, got %#v", "test\n", str)
	}

	<-wait
}
