// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package transform_test

import (
	"fmt"
	"unicode"

	"gx/ipfs/QmVcxhXDbXjNoAdmYBWbY1eU67kQ8eZUHjG4mAYZUtZZu3/go-text/transform"
	"gx/ipfs/QmVcxhXDbXjNoAdmYBWbY1eU67kQ8eZUHjG4mAYZUtZZu3/go-text/unicode/norm"
)

func ExampleRemoveFunc() {
	input := []byte(`tschüß; до свидания`)

	b := make([]byte, len(input))

	t := transform.RemoveFunc(unicode.IsSpace)
	n, _, _ := t.Transform(b, input, true)
	fmt.Println(string(b[:n]))

	t = transform.RemoveFunc(func(r rune) bool {
		return !unicode.Is(unicode.Latin, r)
	})
	n, _, _ = t.Transform(b, input, true)
	fmt.Println(string(b[:n]))

	n, _, _ = t.Transform(b, norm.NFD.Bytes(input), true)
	fmt.Println(string(b[:n]))

	// Output:
	// tschüß;досвидания
	// tschüß
	// tschuß
}
