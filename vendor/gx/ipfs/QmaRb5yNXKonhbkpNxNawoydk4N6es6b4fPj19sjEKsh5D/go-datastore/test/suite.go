package dstest

import (
	"reflect"
	"runtime"
	"testing"

	dstore "gx/ipfs/QmaRb5yNXKonhbkpNxNawoydk4N6es6b4fPj19sjEKsh5D/go-datastore"
)

// BasicSubtests is a list of all basic tests.
var BasicSubtests = []func(t *testing.T, ds dstore.Datastore){
	SubtestBasicPutGet,
	SubtestNotFounds,
	SubtestManyKeysAndQuery,
}

// BatchSubtests is a list of all basic batching datastore tests.
var BatchSubtests = []func(t *testing.T, ds dstore.Batching){
	RunBatchTest,
	RunBatchDeleteTest,
}

func getFunctionName(i interface{}) string {
	return runtime.FuncForPC(reflect.ValueOf(i).Pointer()).Name()
}

// SubtestAll tests the given datastore against all the subtests.
func SubtestAll(t *testing.T, ds dstore.Datastore) {
	for _, f := range BasicSubtests {
		t.Run(getFunctionName(f), func(t *testing.T) {
			f(t, ds)
		})
	}
	if _, ok := ds.(dstore.Batching); ok {
		for _, f := range BasicSubtests {
			t.Run(getFunctionName(f), func(t *testing.T) {
				f(t, ds)
			})
		}
	}
}
