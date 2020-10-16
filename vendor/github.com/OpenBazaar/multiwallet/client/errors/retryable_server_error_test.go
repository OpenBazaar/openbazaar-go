package errors_test

import (
	"errors"
	"testing"

	clientErrs "github.com/OpenBazaar/multiwallet/client/errors"
)

func TestIsRetryable(t *testing.T) {
	var (
		nonRetryable = errors.New("nonretryable error")
		retryable    = clientErrs.NewRetryableError("retryable error")
	)

	if clientErrs.IsRetryable(nonRetryable) {
		t.Error("expected non-retryable error to not indicate retryable, but did")
	}
	if !clientErrs.IsRetryable(retryable) {
		t.Error("expected retryable error to indicate retryable, but did not")
	}
}
