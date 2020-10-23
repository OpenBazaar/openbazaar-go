package wishfix

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"strings"

	"github.com/warpfork/go-wish"
)

// MarshalHunks writes out wishfix.Hunks in a deterministic way.
func MarshalHunks(w io.Writer, h Hunks) error {
	// Write file header.
	w.Write(wordPoundSpace)
	w.Write([]byte(h.title))
	w.Write(wordLF)
	w.Write(wordLF)
	w.Write(wordSectionBreak)
	w.Write(wordLF)

	// Write each section.
	for _, section := range h.sections {
		// Title
		w.Write(wordPoundSpace)
		w.Write([]byte(section.title))
		w.Write(wordLF)
		// Comments (optionally)
		if section.comment != "" {
			lines := strings.Split(section.comment, "\n")
			for _, line := range lines {
				w.Write(wordPoundPoundSpace)
				w.Write([]byte(line))
				w.Write(wordLF)
			}
		}
		// Gap before body
		w.Write(wordLF)

		// Body
		w.Write(wish.IndentBytes(section.body))
		w.Write(wordLF)

		// Always a trailing section break.
		//  (though note the parser is forgiving about this.)
		w.Write(wordSectionBreak)
		w.Write(wordLF)
	}

	return nil
}

// UnmarshalHunks reads and parses a wishfix.Hunk object.
//
// UnmarshalHunks is roughly the dual of MarshalHunks, but note that
// it follows Postel's Law -- be liberal in what you accept -- and is
// significantly more tolerant than MarshalHunks.  Many variations in
// whitespace will be parsed without complaint.
func UnmarshalHunks(r io.Reader) (*Hunks, error) {
	bs, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	lines := bytes.Split(bs, wordLF)
	max := len(lines)
	if max == 0 {
		return &Hunks{}, nil
	}

	h := &Hunks{}
	// First hunk gets slightly special treatment.
	if title, ok := lineIsTitle(lines[0]); ok {
		h.title = string(title)
	} else {
		return h, fmt.Errorf("error on line %d: first line of file must be a title (e.g. `# title`)", 1)
	}
	i := 1
	if i >= max {
		return h, nil
	}
	// Skip up to past the first section break.
	for !lineIsSectionBreak(lines[i]) {
		i++
		if i >= max {
			return h, nil
		}
		//		fmt.Printf("%d: %q\n", i+1, lines[i])
	}
	i++

	// Loop over hunks.
	var sect *section
	for {
		// Slurp any blank lines until we hit a title.
		for len(lines[i]) == 0 {
			i++
			if i >= max {
				return h, nil
			}
		}

		// The first part of a hunk *must* be a title.
		if title, ok := lineIsTitle(lines[i]); ok {
			h.sections = append(h.sections, section{})
			sect = &h.sections[len(h.sections)-1]
			sect.title = string(title)
			i++
		} else {
			return h, fmt.Errorf("error on line %d: first line of each section must be a title (e.g. `# title`)", i+1)
		}
		if i >= max {
			return h, nil
		}

		// Comments now would be fine.
		commentBuf := bytes.Buffer{}
		for {
			if bs, ok := lineIsComment(lines[i]); ok {
				commentBuf.Write(bytes.TrimSpace(bs))
				commentBuf.WriteByte('\n')
				i++
				if i >= max {
					break
				}
			} else {
				break
			}
		}
		sect.comment = commentBuf.String()
		if i >= max {
			return h, nil
		}

		// Slurp any blank lines until we hit body.
		for len(lines[i]) == 0 {
			i++
			if i >= max {
				return h, nil
			}
		}

		// Body begins.  This is *supposed* to be consistently tab-indented,
		//  but we're actually *very* forgiving.  We'll scan straight to the
		//   next section break, and take that whole thing.
		bodyStart := i
		bodyEnd := i
		// Look ahead to section break (or, end).
		//  Also count where we last saw a non-blank line; we'll trim.
		for !lineIsSectionBreak(lines[i]) {
			if len(lines[i]) > 0 {
				bodyEnd++
			}
			i++
			if i >= max {
				break
			}
		}
		//fmt.Printf("body is on lines %d:%d (next section break on %d)\n", i+1, k+1, j+1)
		bodyLines := lines[bodyStart:bodyEnd]
		body := bytes.Join(bodyLines, wordLF) // mem ineffic
		body = append(body, '\n')
		sect.body = wish.DedentBytes(body)
		i++
		if i >= max {
			return h, nil
		}
	}

	return h, nil
}

func lineIsTitle(line []byte) ([]byte, bool) {
	p := len(line) >= 3 && line[0] == '#' && line[1] == ' '
	if !p {
		return nil, false
	}
	return line[2:], true
}

func lineIsComment(line []byte) ([]byte, bool) {
	switch len(line) {
	case 0, 1:
		return nil, false
	case 2:
		return nil, line[0] == '#' && line[1] == '#'
	case 3:
		return nil, line[0] == '#' && line[1] == '#' && line[2] == ' '
	default:
		return line[3:], line[0] == '#' && line[1] == '#' && line[2] == ' '
	}
}

func lineIsSectionBreak(line []byte) bool {
	return len(line) == 3 && line[0] == '-' && line[1] == '-' && line[2] == '-'
}

var (
	wordLF              = []byte{'\n'}
	wordPoundSpace      = []byte("# ")  // section headers
	wordPoundPoundSpace = []byte("## ") // comments
	wordSectionBreak    = []byte("---")
)
