package net

import (
	"context"
	"testing"
	"time"
)

func TestDefaultTimeout(t *testing.T) {
	ctx := context.Background()
	dur := GetDialPeerTimeout(ctx)
	if dur != DialPeerTimeout {
		t.Fatal("expected default peer timeout")
	}
}

func TestNonDefaultTimeout(t *testing.T) {
	customTimeout := time.Duration(1)
	ctx := context.WithValue(
		context.Background(),
		dialPeerTimeoutCtxKey{},
		customTimeout,
	)
	dur := GetDialPeerTimeout(ctx)
	if dur != customTimeout {
		t.Fatal("peer timeout doesn't match set timeout")
	}
}

func TestSettingTimeout(t *testing.T) {
	customTimeout := time.Duration(1)
	ctx := WithDialPeerTimeout(
		context.Background(),
		customTimeout,
	)
	dur := GetDialPeerTimeout(ctx)
	if dur != customTimeout {
		t.Fatal("peer timeout doesn't match set timeout")
	}
}
