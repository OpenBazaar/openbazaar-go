package rsrch

import (
	"fmt"
	"reflect"
	"testing"
)

func TestReflectInvariants(t *testing.T) {
	describe := func(thing interface{}) {
		rt := reflect.TypeOf(thing)
		fmt.Printf("type: %#v (%q)\n", rt, rt)
		fmt.Printf("kind: %#v (%q)\n", rt.Kind(), rt.Kind())
		fmt.Printf("name: %q\n", rt.Name())
		fmt.Printf("PkgPath: %q\n", rt.PkgPath())
	}
	describe("")
	fmt.Println()
	type StrTypedef string
	describe(StrTypedef(""))
	fmt.Println()

	// I guess the cheapest and sanest way to check if something is one of the
	// builtin primitives is just checking the rtid pointer equality outright?
	//
	// I think `PkgPath() == "" && (... kind is not slice, etc...)` might also work,
	// but the documentation doesn't make it very clear if that's an intended use
	// of the PkgPath function, and this seems much harder and less reliable
	// than the "hack".

	fmt.Printf("str rtid: %v\n", reflect.ValueOf(reflect.TypeOf("")).Pointer()) // Strings are ofc consistent.
	fmt.Printf("str rtid: %v\n", reflect.ValueOf(reflect.TypeOf("")).Pointer())
	fmt.Printf("typedstr rtid: %v\n", reflect.ValueOf(reflect.TypeOf(StrTypedef(""))).Pointer()) // Typedefs are consistent, and neq builtin string.
	fmt.Printf("typedstr rtid: %v\n", reflect.ValueOf(reflect.TypeOf(StrTypedef(""))).Pointer())
	fmt.Printf("nillarystruct rtid: %v\n", reflect.ValueOf(reflect.TypeOf(struct{}{})).Pointer()) // Nillary structs are consistent.
	fmt.Printf("nillarystruct rtid: %v\n", reflect.ValueOf(reflect.TypeOf(struct{}{})).Pointer())
	fmt.Printf("anonstruct rtid: %v\n", reflect.ValueOf(reflect.TypeOf(struct{ string }{})).Pointer()) // Anon structs with members are, remarkably, all considered the same.
	fmt.Printf("anonstruct rtid: %v\n", reflect.ValueOf(reflect.TypeOf(struct{ string }{})).Pointer())
	fmt.Printf("[]byte  rtid: %v\n", reflect.ValueOf(reflect.TypeOf([]byte{})).Pointer()) // byte slice and uint8 slice are an alias.
	fmt.Printf("[]uint8 rtid: %v\n", reflect.ValueOf(reflect.TypeOf([]uint8(nil))).Pointer())
}

func TestMapSetSemanitcsLiterallyEven(t *testing.T) {
	anInt := 0
	rv := reflect.ValueOf(anInt)
	anMap := make(map[string]int)
	map_rv := reflect.ValueOf(anMap)
	key := "asdf"
	key_rv := reflect.ValueOf(key)
	map_rv.SetMapIndex(key_rv, rv)
	fmt.Printf("themap: %v\n", anMap)
}
