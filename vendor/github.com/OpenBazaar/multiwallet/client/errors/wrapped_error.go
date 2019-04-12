package errors

type wrappedError interface {
	error
	internalError() error
}

// IsFatal can detect whether a wrappedError is Fatal or not. This
// method does not work on a regular error and will assume all regular
// error instances are not fatal
func IsFatal(err error) bool {
	var ok, iOK bool
	_, ok = err.(FatalServerError)
	if wErr, wOK := err.(wrappedError); wOK {
		iOK = IsFatal(wErr.internalError())
	}
	return ok || iOK
}

// IsRetryable can detect whether a wrappedError is Retryable or not. This
// method does not work on a regular error and will assume all regular
// error instances are not retryable
func IsRetryable(err error) bool {
	var ok, iOK bool
	_, ok = err.(RetryableServerError)
	if wErr, wOK := err.(wrappedError); wOK {
		iOK = IsRetryable(wErr.internalError())
	}
	return ok || iOK
}
