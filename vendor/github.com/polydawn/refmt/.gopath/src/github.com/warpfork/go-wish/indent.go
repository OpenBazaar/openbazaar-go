package wish

import (
	"bytes"
)

// Indent prepends one tab character to each line of a string.
func Indent(s string) string {
	return string(IndentBytes([]byte(s)))
}

// IndentBytes is identically to Indent, but works on a byte slice.
func IndentBytes(bs []byte) []byte {
	if len(bs) == 0 {
		return []byte{'\t'}
	}
	lines := bytes.SplitAfter(bs, []byte{'\n'})
	buf := bytes.Buffer{}
	for _, line := range lines {
		if len(line) > 0 {
			buf.WriteByte('\t')
		}
		buf.Write(line)
	}
	return buf.Bytes()
}

// Dedent strips leading tabs from every line of a string, taking a hint of
// how many tabs should be stripped from the number of consecutive tabs found
// on the first non-empty line.  Dedent also strips one leading blank
// line if it contains nothing but the linebreak.
//
// If later lines have fewer leading tab characters than the depth we intuited
// from the first line, then stripping will still only remove tab characters.
//
// Roughly, Dedent is "Do What I Mean" to normalize a heredoc string
// that contains leading indentation to make it congruent with the
// surrounding source code.
func Dedent(s string) string {
	return string(DedentBytes([]byte(s)))
}

// DedentBytes is identically to Dedent, but works on a byte slice.
func DedentBytes(bs []byte) []byte {
	lines := bytes.SplitAfter(bs, []byte{'\n'})
	buf := bytes.Buffer{}
	if len(lines[0]) == 1 && lines[0][0] == '\n' {
		lines = lines[1:]
	}
	if len(lines) == 0 {
		return []byte{}
	}
	depth := 0
	for _, r := range lines[0] {
		depth++
		if r != '\t' {
			depth--
			break
		}
	}
	for _, line := range lines {
		for i, r := range line {
			if i < depth && r == '\t' {
				continue
			}
			buf.Write(line[i:])
			break
		}
	}
	return buf.Bytes()
}
