package errors_test

import (
	"errors"
	"testing"

	clientErr "github.com/OpenBazaar/multiwallet/client/errors"
)

func TestWrappedErrorsAreComposable(t *testing.T) {
	var (
		baseErr      = errors.New("base")
		fatalErr     = clientErr.MakeFatal(baseErr)
		retryableErr = clientErr.MakeRetryable(baseErr)

		fatalRetryableErr = clientErr.MakeFatal(retryableErr)
		retryableFatalErr = clientErr.MakeRetryable(fatalErr)
	)

	if !clientErr.IsRetryable(fatalRetryableErr) {
		t.Errorf("expected fatal(retryable(err)) to be retryable but was not")
	}
	if !clientErr.IsFatal(fatalRetryableErr) {
		t.Errorf("expected fatal(retryable(err)) to be fatal but was not")
	}

	if !clientErr.IsRetryable(retryableFatalErr) {
		t.Errorf("expected retryable(fatal(err)) to be retryable but was not")
	}
	if !clientErr.IsFatal(retryableFatalErr) {
		t.Errorf("expected retryable(fatal(err)) to be fatal but was not")
	}
}
