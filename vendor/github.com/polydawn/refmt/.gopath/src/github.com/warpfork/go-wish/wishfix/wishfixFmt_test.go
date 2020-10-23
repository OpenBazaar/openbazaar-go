package wishfix

import (
	"bytes"
	"testing"

	"github.com/warpfork/go-wish"
)

var exampleFile = wish.Dedent(`
	# file header

	---
	# section foobar

		{
			"woo": "zow",
			"indentation": "obviously preserved",
			"json": ["not special"]
		}

	---
	# section baz
	## this will be a comment

		it's all just
		like, free text
		maaaan

	---
`)

func TestMarshal(t *testing.T) {
	buf := bytes.Buffer{}
	err := MarshalHunks(&buf, Hunks{
		title: "file header",
		sections: []section{
			{title: "section foobar", body: []byte("{\n\t\"woo\": \"zow\",\n\t\"indentation\": \"obviously preserved\",\n\t\"json\": [\"not special\"]\n}\n")},
			{title: "section baz", comment: "this will be a comment", body: []byte("it's all just\nlike, free text\nmaaaan\n")},
		},
	})
	wish.Wish(t, err, wish.ShouldEqual, nil)
	wish.Wish(t, buf.String(), wish.ShouldEqual, exampleFile)
}

func TestUnmarshal(t *testing.T) {
	buf := bytes.NewBufferString(exampleFile)
	hunks, err := UnmarshalHunks(buf)
	wish.Wish(t, err, wish.ShouldEqual, nil)
	wish.Wish(t, hunks.title, wish.ShouldEqual, "file header")
	wish.Wish(t, hunks.GetSectionList(), wish.ShouldEqual, []string{"section foobar", "section baz"})
	wish.Wish(t, string(hunks.GetSection("section foobar")), wish.ShouldEqual, "{\n\t\"woo\": \"zow\",\n\t\"indentation\": \"obviously preserved\",\n\t\"json\": [\"not special\"]\n}\n")
	wish.Wish(t, hunks.GetSectionComment("section foobar"), wish.ShouldEqual, "")
	wish.Wish(t, string(hunks.GetSection("section baz")), wish.ShouldEqual, "it's all just\nlike, free text\nmaaaan\n")
	wish.Wish(t, hunks.GetSectionComment("section baz"), wish.ShouldEqual, "this will be a comment\n")
}

func TestUnmarshalNittyGritty(t *testing.T) {
	t.Run("empty files", func(t *testing.T) {
		buf := bytes.NewBufferString("")
		_, err := UnmarshalHunks(buf)
		wish.Wish(t, err.Error(), wish.ShouldEqual, "error on line 1: first line of file must be a title (e.g. `# title`)")
	})
	t.Run("solo header no linebreak", func(t *testing.T) {
		buf := bytes.NewBufferString("# whee")
		hunks, err := UnmarshalHunks(buf)
		wish.Wish(t, err, wish.ShouldEqual, nil)
		wish.Wish(t, hunks.title, wish.ShouldEqual, "whee")
	})
	t.Run("solo header many linebreak", func(t *testing.T) {
		buf := bytes.NewBufferString("# whee\n\n\n\n")
		hunks, err := UnmarshalHunks(buf)
		wish.Wish(t, err, wish.ShouldEqual, nil)
		wish.Wish(t, hunks.title, wish.ShouldEqual, "whee")
	})
	t.Run("solo header immediate section", func(t *testing.T) {
		buf := bytes.NewBufferString("# whee\n---\n\n\n")
		hunks, err := UnmarshalHunks(buf)
		wish.Wish(t, err, wish.ShouldEqual, nil)
		wish.Wish(t, hunks.title, wish.ShouldEqual, "whee")
	})
	t.Run("solo header immediate section", func(t *testing.T) {
		buf := bytes.NewBufferString("# whee\n\n\n\n---\n\n\n")
		hunks, err := UnmarshalHunks(buf)
		wish.Wish(t, err, wish.ShouldEqual, nil)
		wish.Wish(t, hunks.title, wish.ShouldEqual, "whee")
	})
	t.Run("gap between section and title", func(t *testing.T) {
		buf := bytes.NewBufferString("# whee\n---\n\n\n# title")
		hunks, err := UnmarshalHunks(buf)
		wish.Wish(t, err, wish.ShouldEqual, nil)
		wish.Wish(t, hunks.title, wish.ShouldEqual, "whee")
		wish.Wish(t, hunks.GetSectionList(), wish.ShouldEqual, []string{"title"})
	})
	t.Run("comments fly right", func(t *testing.T) {
		t.Run("single line", func(t *testing.T) {
			buf := bytes.NewBufferString("# whee\n---\n# title\n## comment\n")
			hunks, err := UnmarshalHunks(buf)
			wish.Wish(t, err, wish.ShouldEqual, nil)
			wish.Wish(t, hunks.title, wish.ShouldEqual, "whee")
			wish.Wish(t, hunks.GetSectionList(), wish.ShouldEqual, []string{"title"})
			wish.Wish(t, hunks.GetSectionComment("title"), wish.ShouldEqual, "comment\n")
		})
	})
	t.Run("title with no space is BS", func(t *testing.T) {
		buf := bytes.NewBufferString("# whee\n---\n#title\n## comment\n")
		_, err := UnmarshalHunks(buf)
		wish.Wish(t, err.Error(), wish.ShouldEqual, "error on line 3: first line of each section must be a title (e.g. `# title`)")
	})
	t.Run("gap before section comment isn't a comment", func(t *testing.T) { // oddly, our "tolerance" modes mean it's a body now.
		buf := bytes.NewBufferString("# whee\n---\n# title\n\n## comment\n")
		hunks, err := UnmarshalHunks(buf)
		wish.Wish(t, err, wish.ShouldEqual, nil)
		wish.Wish(t, hunks.title, wish.ShouldEqual, "whee")
		wish.Wish(t, hunks.GetSectionList(), wish.ShouldEqual, []string{"title"})
		wish.Wish(t, hunks.GetSectionComment("title"), wish.ShouldEqual, "")
		wish.Wish(t, string(hunks.GetSection("title")), wish.ShouldEqual, "## comment\n")
	})
	t.Run("comments right up to eof are fine", func(t *testing.T) {
		buf := bytes.NewBufferString("# whee\n---\n# title\n## comment\n## comment\n## comment")
		hunks, err := UnmarshalHunks(buf)
		wish.Wish(t, err, wish.ShouldEqual, nil)
		wish.Wish(t, hunks.title, wish.ShouldEqual, "whee")
		wish.Wish(t, hunks.GetSectionList(), wish.ShouldEqual, []string{"title"})
		wish.Wish(t, hunks.GetSectionComment("title"), wish.ShouldEqual, "comment\ncomment\ncomment\n")
		wish.Wish(t, hunks.GetSection("title") == nil, wish.ShouldEqual, false)
		wish.Wish(t, string(hunks.GetSection("title")), wish.ShouldEqual, "")
	})
	t.Run("body with normal indentation flies right", func(t *testing.T) {
		buf := bytes.NewBufferString(wish.Dedent(`
			# whee
			---
			# title
			
				body
				body
			
		`))
		hunks, err := UnmarshalHunks(buf)
		wish.Wish(t, err, wish.ShouldEqual, nil)
		wish.Wish(t, hunks.title, wish.ShouldEqual, "whee")
		wish.Wish(t, hunks.GetSectionList(), wish.ShouldEqual, []string{"title"})
		wish.Wish(t, hunks.GetSectionComment("title"), wish.ShouldEqual, "")
		wish.Wish(t, string(hunks.GetSection("title")), wish.ShouldEqual, "body\nbody\n")
	})
	t.Run("body with no final section break is tolerated", func(t *testing.T) {
		buf := bytes.NewBufferString("# whee\n---\n# title\n\nbody\nbody\n")
		hunks, err := UnmarshalHunks(buf)
		wish.Wish(t, err, wish.ShouldEqual, nil)
		wish.Wish(t, hunks.title, wish.ShouldEqual, "whee")
		wish.Wish(t, hunks.GetSectionList(), wish.ShouldEqual, []string{"title"})
		wish.Wish(t, hunks.GetSectionComment("title"), wish.ShouldEqual, "")
		wish.Wish(t, string(hunks.GetSection("title")), wish.ShouldEqual, "body\nbody\n")
	})
	t.Run("body with no final line break tolerated", func(t *testing.T) { // also borderline odd.
		buf := bytes.NewBufferString("# whee\n---\n# title\n\nbody\nbody")
		hunks, err := UnmarshalHunks(buf)
		wish.Wish(t, err, wish.ShouldEqual, nil)
		wish.Wish(t, hunks.title, wish.ShouldEqual, "whee")
		wish.Wish(t, hunks.GetSectionList(), wish.ShouldEqual, []string{"title"})
		wish.Wish(t, hunks.GetSectionComment("title"), wish.ShouldEqual, "")
		wish.Wish(t, string(hunks.GetSection("title")), wish.ShouldEqual, "body\nbody\n")
	})
	t.Run("body that runs too close to section edge tolerated", func(t *testing.T) {
		buf := bytes.NewBufferString("# whee\n---\n# title\nbody\nbody\n---\n# second section\nit's fine\n")
		hunks, err := UnmarshalHunks(buf)
		wish.Wish(t, err, wish.ShouldEqual, nil)
		wish.Wish(t, hunks.title, wish.ShouldEqual, "whee")
		wish.Wish(t, hunks.GetSectionList(), wish.ShouldEqual, []string{"title", "second section"})
		wish.Wish(t, hunks.GetSectionComment("title"), wish.ShouldEqual, "")
		wish.Wish(t, string(hunks.GetSection("title")), wish.ShouldEqual, "body\nbody\n")
		wish.Wish(t, hunks.GetSectionComment("second section"), wish.ShouldEqual, "")
		wish.Wish(t, string(hunks.GetSection("second section")), wish.ShouldEqual, "it's fine\n")
	})
	t.Run("sections with empty body are blank and not nil", func(t *testing.T) {
		test := func(t *testing.T, buf *bytes.Buffer) {
			hunks, err := UnmarshalHunks(buf)
			wish.Wish(t, err, wish.ShouldEqual, nil)
			wish.Wish(t, hunks.title, wish.ShouldEqual, "whee")
			wish.Wish(t, hunks.GetSectionList(), wish.ShouldEqual, []string{"section"})
			wish.Wish(t, hunks.GetSectionComment("section"), wish.ShouldEqual, "")
			wish.Wish(t, hunks.GetSection("section") == nil, wish.ShouldEqual, false)
			wish.Wish(t, string(hunks.GetSection("section")), wish.ShouldEqual, "")
		}
		t.Run("when the section is empty", func(t *testing.T) {
			test(t, bytes.NewBufferString(wish.Dedent(`
				# whee
				---
				# section
			`)))
		})
		t.Run("when the section has trailing separator", func(t *testing.T) {
			test(t, bytes.NewBufferString(wish.Dedent(`
				# whee
				---
				# section
				---
			`)))
		})
		t.Run("when the section has trailing separator and further trailing space", func(t *testing.T) {
			test(t, bytes.NewBufferString(wish.Dedent(`
				# whee
				---
				# section
				---
				
				
			`)))
		})
		t.Run("when the section has gap and trailing separator", func(t *testing.T) {
			test(t, bytes.NewBufferString(wish.Dedent(`
				# whee
				---
				# section
				
				
				---
			`)))
		})
		t.Run("when the section has gap and no trailing separator", func(t *testing.T) {
			test(t, bytes.NewBufferString(wish.Dedent(`
				# whee
				---
				# section
				
				
			`)))
		})
	})
}
