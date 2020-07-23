package basicnode

// Style embeds a NodeStyle for every kind of Node implementation in this package.
// You can use it like this:
//
// 		basicnode.Style.Map.NewBuilder().BeginMap() //...
//
// and:
//
// 		basicnode.Style.String.NewBuilder().AssignString("x") // ...
//
// Most of the styles here are for one particular Kind of node (e.g. string, int, etc);
// you can use the "Any" style if you want a builder that can accept any kind of data.
var Style style

type style struct {
	Any    Style__Any
	Map    Style__Map
	List   Style__List
	Bool   Style__Bool
	Int    Style__Int
	Float  Style__Float
	String Style__String
	Bytes  Style__Bytes
	Link   Style__Link
}
