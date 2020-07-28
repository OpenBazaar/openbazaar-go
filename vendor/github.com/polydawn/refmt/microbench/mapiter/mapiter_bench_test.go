package bench

import (
	"encoding/json"
	"io/ioutil"
	"reflect"
	"strconv"
	"testing"
)

// Iterate a map by ranging.
func Benchmark_MapIterDirectRange(b *testing.B) {
	var slot map[string]interface{}
	var dropK string
	var dropV interface{}
	slot = mapItrFixture()
	for i := 0; i < b.N; i++ {
		for k, v := range slot {
			dropK = k
			dropV = v
		}
	}
	_, _ = dropK, dropV
}

// Iterate a map by indexing in for each key.
// This is not using reflect, but does involve additional time probing into the map;
//  this emulates the work necessary if accessing the map in key-sorted order (but does
//  not include costs of such a sort).
//
// About 3~4x slower than direct range.
// Almost no allocs (one! just that big slice for the keys).
func Benchmark_MapIterDirectByKeys(b *testing.B) {
	var slot map[string]interface{}
	var dropK string
	var dropV interface{}
	slot = mapItrFixture()
	for i := 0; i < b.N; i++ {
		ks := make([]string, len(slot))
		var j int
		for k, _ := range slot {
			ks[j] = k
			j++
		}
		for _, k := range ks {
			dropV = slot[k]
			dropK = k
		}
	}
	_, _ = dropK, dropV
}

// Iterate a map by reflection.
// This is (necessarily) comparable to IterDirectByKeys, since there is no such thing
//  as `reflect.Value.Range()` (and if there was, it would probably have to take a callback),
//  so we're getting keys first, and indexing in per key.
//
// About 16x slower than direct range.  About 4~5x slower than direct keys.
// About 2 allocs per map entry.
func Benchmark_MapIterReflective(b *testing.B) {
	var slot map[string]interface{}
	var dropK string
	var dropV interface{}
	slot = mapItrFixture()
	rv := reflect.ValueOf(slot)
	for i := 0; i < b.N; i++ {
		keys := rv.MapKeys()
		for _, k := range keys {
			dropK = k.Interface().(string)
			dropV = rv.MapIndex(k).Interface()
		}
	}
	_, _ = dropK, dropV
}

// Exercise the stdlib json encoding traversing the same map fixture.
// This can be expected to be crazy slow compared to the other tests, because
//  it has to do tons of work shifting bytes around.
//
// About 2.3x slower than reflective iteration.  (Yup, that means about 37x slower than direct ranging.)
// About 3 allocs per map entry.
func Benchmark_MapIterContextJson(b *testing.B) {
	var slot map[string]interface{}
	slot = mapItrFixture()
	for i := 0; i < b.N; i++ {
		json.NewEncoder(ioutil.Discard).Encode(slot)
	}
}

func mapItrFixture() map[string]interface{} {
	slot := make(map[string]interface{}, 1000)
	for i := 1000; i > 0; i-- {
		slot[strconv.Itoa(i)] = i
	}
	return slot
}
