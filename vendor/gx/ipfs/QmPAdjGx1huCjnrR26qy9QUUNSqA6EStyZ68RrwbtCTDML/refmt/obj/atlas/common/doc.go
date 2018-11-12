/*
	commonatlases is a package full of `atlas.Entry` definions for common
	types in the standard library.

	A frequently useful example is the standard library `time.Time` type,
	which is frequently serialized in some custom way: as a unix int for
	example, or perhaps an RFC3339 string; there are many popular choices.
	(`time.Time` is also an example of where *some* custom behavior is
	pretty much required, because a default struct-mapping is useless on
	a struct with no exported fields.)
*/
package commonatlases
