package jwt

// Resolver is an Algorithm that needs to set some variables
// based on a Header before performing signing and verification.
type Resolver interface {
	Resolve(Header) error
}
