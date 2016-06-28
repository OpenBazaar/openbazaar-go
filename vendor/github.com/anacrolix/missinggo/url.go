package missinggo

import "net/url"

// Returns URL opaque as an unrooted path.
func URLOpaquePath(u *url.URL) string {
	if u.Opaque != "" {
		return u.Opaque
	}
	return u.Path
}
