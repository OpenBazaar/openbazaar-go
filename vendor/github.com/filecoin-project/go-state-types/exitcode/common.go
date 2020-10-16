package exitcode

// Common error codes that may be shared by different actors.
// Actors may also define their own codes, including redefining these values.

const (
	// Indicates a method parameter is invalid.
	ErrIllegalArgument = FirstActorErrorCode + iota
	// Indicates a requested resource does not exist.
	ErrNotFound
	// Indicates an action is disallowed.
	ErrForbidden
	// Indicates a balance of funds is insufficient.
	ErrInsufficientFunds
	// Indicates an actor's internal state is invalid.
	ErrIllegalState
	// Indicates de/serialization failure within actor code.
	ErrSerialization

	// Common error codes stop here.  If you define a common error code above
	// this value it will have conflicting interpretations
	FirstActorSpecificExitCode = ExitCode(32)
)
