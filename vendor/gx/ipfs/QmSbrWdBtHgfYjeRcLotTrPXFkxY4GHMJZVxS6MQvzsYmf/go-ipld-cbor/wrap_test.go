package cbornode

import (
	"sync"
	"testing"

	mh "gx/ipfs/QmPnFwZ2JXKnXgMw8CdBPxn7FWh6LLdjUjxV1fKHuJnkr8/go-multihash"
)

type MyStruct struct {
	Items map[string]MyStruct
	Foo   string
	Bar   []byte
	Baz   []int
}

func init() {
	RegisterCborType(MyStruct{})
}

func testStruct() MyStruct {
	return MyStruct{
		Items: map[string]MyStruct{
			"Foo": {
				Foo: "Foo",
				Bar: []byte("Bar"),
				Baz: []int{1, 2, 3, 4},
			},
			"Bar": {
				Bar: []byte("Bar"),
				Baz: []int{1, 2, 3, 4},
			},
		},
		Baz: []int{5, 1, 2},
	}
}

func BenchmarkWrapObject(b *testing.B) {
	obj := testStruct()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		nd, err := WrapObject(obj, mh.SHA2_256, -1)
		if err != nil {
			b.Fatal(err, nd)
		}
	}
}

func BenchmarkDecodeBlock(b *testing.B) {
	obj := testStruct()
	nd, err := WrapObject(obj, mh.SHA2_256, -1)
	if err != nil {
		b.Fatal(err, nd)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		nd2, err := DecodeBlock(nd)
		if err != nil {
			b.Fatal(err, nd2)
		}
	}
}

func BenchmarkWrapObjectParallel(b *testing.B) {
	obj := testStruct()
	b.ResetTimer()
	var wg sync.WaitGroup
	wg.Add(100)
	for j := 0; j < 100; j++ {
		go func() {
			defer wg.Done()
			for i := 0; i < b.N; i++ {
				nd, err := WrapObject(obj, mh.SHA2_256, -1)
				if err != nil {
					b.Fatal(err, nd)
				}
			}
		}()
	}
	wg.Wait()
}

func BenchmarkDecodeBlockParallel(b *testing.B) {
	obj := testStruct()
	nd, err := WrapObject(obj, mh.SHA2_256, -1)
	if err != nil {
		b.Fatal(err, nd)
	}
	b.ResetTimer()
	var wg sync.WaitGroup
	wg.Add(100)
	for j := 0; j < 100; j++ {
		go func() {
			defer wg.Done()
			for i := 0; i < b.N; i++ {
				nd2, err := DecodeBlock(nd)
				if err != nil {
					b.Fatal(err, nd2)
				}
			}
		}()
	}
	wg.Wait()
}

func BenchmarkDumpObject(b *testing.B) {
	obj := testStruct()
	for i := 0; i < b.N; i++ {
		bytes, err := DumpObject(obj)
		if err != nil {
			b.Fatal(err, bytes)
		}
	}
}
