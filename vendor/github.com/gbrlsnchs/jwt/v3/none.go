package jwt

var _ Algorithm = none{}

type none struct{}

// None returns a dull, unsecured algorithm.
func None() Algorithm { return none{} }

// Name always returns "none".
func (none) Name() string { return "none" }

// Sign always returns a nil byte slice and a nil error.
func (none) Sign(_ []byte) ([]byte, error) { return nil, nil }

// Size always returns 0 and a nil error.
func (none) Size() int { return 0 }

// Verify always returns a nil error.
func (none) Verify(_, _ []byte) error { return nil }
