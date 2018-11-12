package levenshtein

import (
	"fmt"
	"os"
	"testing"
)

var testCases = []struct {
	source   string
	target   string
	distance int
	script   EditScript
}{
	{"", "a", 1, EditScript{Ins}},
	{"a", "aa", 1, EditScript{Match, Ins}},
	{"a", "aaa", 2, EditScript{Match, Ins, Ins}},
	{"", "", 0, EditScript{}},
	{"a", "b", 2, EditScript{Ins, Del}},
	{"aaa", "aba", 2, EditScript{Match, Ins, Match, Del}},
	{"aaa", "ab", 3, EditScript{Match, Ins, Del, Del}},
	{"a", "a", 0, EditScript{Match}},
	{"ab", "ab", 0, EditScript{Match, Match}},
	{"a", "", 1, EditScript{Del}},
	{"aa", "a", 1, EditScript{Match, Del}},
	{"aaa", "a", 2, EditScript{Match, Del, Del}},
}

func TestDistanceForStrings(t *testing.T) {
	for _, testCase := range testCases {
		distance := DistanceForStrings(
			[]rune(testCase.source),
			[]rune(testCase.target),
			DefaultOptions)
		if distance != testCase.distance {
			t.Log(
				"Distance between",
				testCase.source,
				"and",
				testCase.target,
				"computed as",
				distance,
				", should be",
				testCase.distance)
			t.Fail()
		}
	}
}

func TestEditScriptForStrings(t *testing.T) {
	for _, testCase := range testCases {
		script := EditScriptForStrings(
			[]rune(testCase.source),
			[]rune(testCase.target),
			DefaultOptions)
		if !equal(script, testCase.script) {
			t.Log(
				"Edit script from",
				testCase.source,
				"to",
				testCase.target,
				"computed as",
				script,
				", should be",
				testCase.script)
			t.Fail()
		}
	}
}

func equal(a, b EditScript) bool {
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func ExampleDistanceForStrings() {
	source := "a"
	target := "aa"
	distance := DistanceForStrings([]rune(source), []rune(target), DefaultOptions)
	fmt.Printf(`Distance between "%s" and "%s" computed as %d`, source, target, distance)
	// Output: Distance between "a" and "aa" computed as 1
}

func ExampleWriteMatrix() {
	source := []rune("neighbor")
	target := []rune("Neighbour")
	matrix := MatrixForStrings(source, target, DefaultOptions)
	WriteMatrix(source, target, matrix, os.Stdout)
	// Output:
	//       N  e  i  g  h  b  o  u  r
	//    0  1  2  3  4  5  6  7  8  9
	// n  1  2  3  4  5  6  7  8  9 10
	// e  2  3  2  3  4  5  6  7  8  9
	// i  3  4  3  2  3  4  5  6  7  8
	// g  4  5  4  3  2  3  4  5  6  7
	// h  5  6  5  4  3  2  3  4  5  6
	// b  6  7  6  5  4  3  2  3  4  5
	// o  7  8  7  6  5  4  3  2  3  4
	// r  8  9  8  7  6  5  4  3  4  3
}
