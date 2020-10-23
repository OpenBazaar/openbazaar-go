package wish

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/warpfork/go-wish/difflib"
)

func strdiff(a, b string) string {
	result, err := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
		A:       escapishSlice(strings.SplitAfter(a, "\n")),
		B:       escapishSlice(strings.SplitAfter(b, "\n")),
		Context: 3,
	})
	if err != nil {
		panic(fmt.Errorf("diffing failed: %s", err))
	}
	return result
}

func escapishSlice(ss []string) []string {
	for i, s := range ss {
		ss[i] = EscapeToASCII(s) + "\n"
	}
	return ss
}

const lowerhex = "0123456789abcdef"

// EscapeToASCII returns a string where each rune has been parsed,
// and all unprintable or non-ascii values have been escaped.
// Common escape codes such as '\t' and '\a' are used where possible;
// remaining values are encoded as hex or unicode escapes ('\x', '\u', and '\U').
// Included in the escaped values are '\n'; excluded are spaces and quote marks
// (this latter part being a notable distinction from `strconv.QuoteToASCII`).
//
// go-wish features for string comparison may use EscapeToASCII automatically
// to help highlight whitespace and other hard-to-see differences if it detects
// strings differing only by such values.
func EscapeToASCII(s string) string {
	return string(appendEscaped(make([]byte, 0, 3*len(s)/2), s))
}

// appendEscaped applies appendEscapedRune for each run in the string.
// this is not unlike stdlib `strconv.appendQuotedWith`, but does not
// escape quotation marks.
func appendEscaped(buf []byte, s string) []byte {
	for width := 0; len(s) > 0; s = s[width:] {
		r := rune(s[0])
		width = 1
		if r >= utf8.RuneSelf {
			r, width = utf8.DecodeRuneInString(s)
		}
		if width == 1 && r == utf8.RuneError {
			buf = append(buf, `\x`...)
			buf = append(buf, lowerhex[s[0]>>4])
			buf = append(buf, lowerhex[s[0]&0xF])
			continue
		}
		buf = appendEscapedRune(buf, r)
	}
	return buf
}

// append runes, always escaping above-ascii or invisibles.
// this is not unlike stdlib `strconv.appendEscapedRune`, but does not
// escape quotation marks.
func appendEscapedRune(buf []byte, r rune) []byte {
	// Backslash is always escaped.
	if r == '\\' {
		buf = append(buf, '\\')
		buf = append(buf, '\\')
		return buf
	}
	// ASCII between space and before DEL is printable.
	if 0x20 <= r && r <= 0x7E {
		buf = append(buf, byte(r))
		return buf
	}
	// Switch for explicit handling of the "common" escapes.
	// The default case will hex-encode the remainder.
	switch r {
	case '\a':
		buf = append(buf, `\a`...)
	case '\b':
		buf = append(buf, `\b`...)
	case '\f':
		buf = append(buf, `\f`...)
	case '\n':
		buf = append(buf, `\n`...)
	case '\r':
		buf = append(buf, `\r`...)
	case '\t':
		buf = append(buf, `\t`...)
	case '\v':
		buf = append(buf, `\v`...)
	default:
		switch {
		case r < ' ':
			buf = append(buf, `\x`...)
			buf = append(buf, lowerhex[byte(r)>>4])
			buf = append(buf, lowerhex[byte(r)&0xF])
		case r > utf8.MaxRune:
			r = 0xFFFD
			fallthrough
		case r < 0x10000:
			buf = append(buf, `\u`...)
			for s := 12; s >= 0; s -= 4 {
				buf = append(buf, lowerhex[r>>uint(s)&0xF])
			}
		default:
			buf = append(buf, `\U`...)
			for s := 28; s >= 0; s -= 4 {
				buf = append(buf, lowerhex[r>>uint(s)&0xF])
			}
		}
	}
	return buf
}
