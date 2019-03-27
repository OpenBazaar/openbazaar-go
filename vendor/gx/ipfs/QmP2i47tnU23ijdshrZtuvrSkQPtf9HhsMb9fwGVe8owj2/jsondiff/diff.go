package jsondiff

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"sort"

	"gx/ipfs/QmRCUXvrfEEpWfqkLKqiaXE2uVaX73MGSVjLrfHDmzygTg/ansi"
)

// ResolutionType defines a type of comparison: equality, non-equality,
// new sub-diff and so on
type ResolutionType int

const (
	TypeEquals ResolutionType = iota
	TypeNotEquals
	TypeAdded
	TypeRemoved
	TypeDiff

	indentation = "    "
)

var (
	colorStartYellow = ansi.ColorCode("yellow")
	colorStartRed    = ansi.ColorCode("red")
	colorStartGreen  = ansi.ColorCode("green")
	colorReset       = ansi.ColorCode("reset")
)

// Diff is a result of comparison operation. Provides list
// of items that describe difference between objects piece by piece
type Diff struct {
	items   []DiffItem
	hasDiff bool
}

// Items returns list of diff items
func (d Diff) Items() []DiffItem { return d.items }

// Add adds new item to diff object
func (d *Diff) Add(item DiffItem) {
	d.items = append(d.items, item)
	if item.Resolution != TypeEquals {
		d.hasDiff = true
	}
}

// IsEqual checks if given diff objects does not contain any non-equal
// element. When IsEqual returns "true" that means there is no difference
// between compared objects
func (d Diff) IsEqual() bool { return !d.hasDiff }

func (d *Diff) sort() { sort.Sort(byKey(d.items)) }

// DiffItem defines a difference between 2 items with resolution type
type DiffItem struct {
	Key        string
	ValueA     interface{}
	Resolution ResolutionType
	ValueB     interface{}
}

type byKey []DiffItem

func (m byKey) Len() int           { return len(m) }
func (m byKey) Less(i, j int) bool { return m[i].Key < m[j].Key }
func (m byKey) Swap(i, j int)      { m[i], m[j] = m[j], m[i] }

// Compare produces list of diff items that define difference between
// objects "a" and "b".
// Note: if objects are equal, all diff items will have Resolution of
// type TypeEquals
func Compare(a, b interface{}) Diff {
	mapA := map[string]interface{}{}
	mapB := map[string]interface{}{}

	jsonA, _ := json.Marshal(a)
	jsonB, _ := json.Marshal(b)

	json.Unmarshal(jsonA, &mapA)
	json.Unmarshal(jsonB, &mapB)

	return compareStringMaps(mapA, mapB)
}

// Format produces formatted output for a diff that can be printed.
// Uses colourization which may not work with terminals that don't
// support ASCII colouring (Windows is under question).
func Format(diff Diff) []byte {
	buf := bytes.Buffer{}

	writeItems(&buf, "", diff.Items())

	return buf.Bytes()
}

func writeItems(writer io.Writer, prefix string, items []DiffItem) {
	writer.Write([]byte{'{'})
	last := len(items) - 1

	prefixNotEqualsA := prefix + "<> "
	prefixNotEqualsB := prefix + "** "
	prefixAdded := prefix + "<< "
	prefixRemoved := prefix + ">> "

	for i, item := range items {
		writer.Write([]byte{'\n'})

		switch item.Resolution {
		case TypeEquals:
			writeItem(writer, prefix, item.Key, item.ValueA, i < last)
		case TypeNotEquals:
			writer.Write([]byte(colorStartYellow))

			writeItem(writer, prefixNotEqualsA, item.Key, item.ValueA, i < last)
			writer.Write([]byte{'\n'})
			writeItem(writer, prefixNotEqualsB, item.Key, item.ValueB, i < last)

			writer.Write([]byte(colorReset))
		case TypeAdded:
			writer.Write([]byte(colorStartGreen))
			writeItem(writer, prefixAdded, item.Key, item.ValueB, i < last)
			writer.Write([]byte(colorReset))
		case TypeRemoved:
			writer.Write([]byte(colorStartRed))
			writeItem(writer, prefixRemoved, item.Key, item.ValueA, i < last)
			writer.Write([]byte(colorReset))
		case TypeDiff:
			subdiff := item.ValueB.([]DiffItem)
			fmt.Fprintf(writer, "%s\"%s\": ", prefix, item.Key)
			writeItems(writer, prefix+indentation, subdiff)
			if i < last {
				writer.Write([]byte{','})
			}
		}

	}

	fmt.Fprintf(writer, "\n%s}", prefix)
}

func writeItem(writer io.Writer, prefix, key string, value interface{}, isNotLast bool) {
	fmt.Fprintf(writer, "%s\"%s\": ", prefix, key)
	serialized, _ := json.Marshal(value)

	writer.Write(serialized)
	if isNotLast {
		writer.Write([]byte{','})
	}
}

func compare(A, B interface{}) (ResolutionType, Diff) {
	equals := reflect.DeepEqual(A, B)
	if equals {
		return TypeEquals, Diff{}
	}

	mapA, okA := A.(map[string]interface{})
	mapB, okB := B.(map[string]interface{})

	if okA && okB {
		diff := compareStringMaps(mapA, mapB)
		return TypeDiff, diff
	}

	arrayA, okA := A.([]interface{})
	arrayB, okB := B.([]interface{})

	if okA && okB {
		diff := compareArrays(arrayA, arrayB)
		return TypeDiff, diff
	}

	return TypeNotEquals, Diff{}
}

func compareArrays(A, B []interface{}) Diff {
	result := Diff{}

	minLength := len(A)
	if len(A) > len(B) {
		minLength = len(B)
	}

	for i := 0; i < minLength; i++ {
		resolutionType, subdiff := compare(A[i], B[i])

		switch resolutionType {
		case TypeEquals:
			result.Add(DiffItem{"", A[i], TypeEquals, nil})
		case TypeNotEquals:
			result.Add(DiffItem{"", A[i], TypeNotEquals, B[i]})
		case TypeDiff:
			result.Add(DiffItem{"", nil, TypeDiff, subdiff.Items()})
		}
	}

	for i := minLength; i < len(A); i++ {
		result.Add(DiffItem{"", A[i], TypeRemoved, nil})
	}

	for i := minLength; i < len(B); i++ {
		result.Add(DiffItem{"", nil, TypeAdded, B[i]})
	}

	return result
}

func compareStringMaps(A, B map[string]interface{}) Diff {
	keysA := sortedKeys(A)
	keysB := sortedKeys(B)

	result := Diff{}

	for _, kA := range keysA {
		vA := A[kA]

		vB, ok := B[kA]
		if !ok {
			result.Add(DiffItem{kA, vA, TypeRemoved, nil})
			continue
		}

		resolutionType, subdiff := compare(vA, vB)

		switch resolutionType {
		case TypeEquals:
			result.Add(DiffItem{kA, vA, TypeEquals, nil})
		case TypeNotEquals:
			result.Add(DiffItem{kA, vA, TypeNotEquals, vB})
		case TypeDiff:
			result.Add(DiffItem{kA, nil, TypeDiff, subdiff.Items()})
		}
	}

	for _, kB := range keysB {
		if _, ok := A[kB]; !ok {
			result.Add(DiffItem{kB, nil, TypeAdded, B[kB]})
		}
	}

	result.sort()

	return result
}

func sortedKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	return keys
}
