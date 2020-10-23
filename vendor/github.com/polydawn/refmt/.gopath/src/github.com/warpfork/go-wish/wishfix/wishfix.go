package wishfix

// Hunks is a struct representing the contents of a wishfix-formatted file in memory.
//
// The complete set of hunks has one title, and a list of sections.
// "Sections" is defined per the FORMAT document in this directory.
type Hunks struct {
	title    string
	sections []section
}

type section struct {
	title   string
	comment string
	body    []byte
}

// CreateHunks returns a new, blank hunks with only a title assigned.
func CreateHunks(masterTitle string) Hunks {
	return Hunks{
		title: masterTitle,
	}
}

// GetSectionList returns the titles of all sections in this Hunks.
//
// Note that section order is persistent.
//
// Note that you can also check if an individual section is present by checking
// `GetSection(title) == nil`, so long as you know the title in advance.
func (h Hunks) GetSectionList() (v []string) {
	for _, s := range h.sections {
		v = append(v, s.title)
	}
	return
}

// GetSection returns the body of the section as bytes.
//
// If the section is absent, nil will be returned; in all other cases,
// a byte slice is returned (e.g., even if the section body is completely empty,
// as long as the title exists, a zero-length non-nil slice is returned.)
func (h Hunks) GetSection(title string) []byte {
	for _, s := range h.sections {
		if s.title == title {
			if s.body == nil {
				return []byte{}
			}
			return s.body
		}
	}
	return nil
}

// GetSectionComment returns a string of the section comments
// (see the FORMAT document in this directory for more info on comments).
//
// If there are no comments, or if there's no section with this title,
// an empty string is returned.
func (h Hunks) GetSectionComment(title string) string {
	for _, s := range h.sections {
		if s.title == title {
			return s.comment
		}
	}
	return ""
}

// PutSection assigns a section and its body.
//
// If a section of this title already exists: its body will be updated,
// any comments are unchanged, and the overall ordering of sections is unchanged.
// If there's no section with this title, a new section with this body
// will be appended to the end of the set.
// (Order of sections is preserved and can be inspected via GetSectionList,
// and is also the order in which hunks will be serialized when persisted.)
func (h Hunks) PutSection(title string, body []byte) Hunks {
	for i, s := range h.sections {
		if s.title == title {
			h.sections[i].body = body
			return h
		}
	}
	h.sections = append(h.sections, section{title: title, body: body})
	return h
}
