package errors_test

import (
	"errors"
	"testing"

	clientErrs "github.com/OpenBazaar/multiwallet/client/errors"
)

func TestIsFatal(t *testing.T) {
	var (
		nonFatal = errors.New("nonfatal error")
		fatal    = clientErrs.NewFatalError("fatal error")
	)

	if clientErrs.IsFatal(nonFatal) {
		t.Error("expected non-fatal error to not indicate fatal, but did")
	}
	if !clientErrs.IsFatal(fatal) {
		t.Error("expected fatal error to indicate fatal, but did not")
	}
}
