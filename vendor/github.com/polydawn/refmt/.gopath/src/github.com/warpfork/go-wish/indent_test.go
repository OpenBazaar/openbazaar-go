package wish

import "testing"

func TestIndent(t *testing.T) {
	for _, tr := range []struct{ a, b string }{
		{"", "\t"},
		{"\t", "\t\t"},
		{"a", "\ta"},
		{"a\nb\nc", "\ta\n\tb\n\tc"},
		{"a\nb\n", "\ta\n\tb\n"},
		{"\ta\nb\n\tc\n", "\t\ta\n\tb\n\t\tc\n"},
		{"a\n\nb\n", "\ta\n\t\n\tb\n"},
	} {
		actual := Indent(tr.a)
		if actual != tr.b {
			t.Errorf("Indent(%q) != %q: got %q", tr.a, tr.b, actual)
		}
	}
}

func TestDeIndent(t *testing.T) {
	for _, tr := range []string{
		"",
		"a",
		"a\nb\nc",
		"a\nb\n",
		"a\n\nb\n",
	} {
		actual := Dedent(Indent(tr))
		if actual != tr {
			t.Errorf("Dedent(Indent(%q)) != self: got %q", tr, actual)
		}
	}
}

func TestDedent(t *testing.T) {
	for _, tr := range []struct{ a, b string }{
		{"", ""},
		{"\t", ""},
		{"\t\t", ""},
		{"\n", ""},
		{"\n\t", ""},
		{"\n\t\t", ""},
		{"\n\n", "\n"},
		{"\n\t\n", "\n"},
		{"\n\t\t\n", "\n"},
		{"\n\n", "\n"},
		{"\n\n\t", "\n\t"},
		{"\n\n\t\t", "\n\t\t"},
		{"a\nb\n\tc\n", "a\nb\n\tc\n"},
		{"\ta\nb\n\tc\n", "a\nb\nc\n"},
		{"\t\ta\nb\n\tc\n", "a\nb\nc\n"},
		{"\ta\n\t\tb\n\tc\n", "a\n\tb\nc\n"},
		{"\ta\n\t\t\tb\n\tc\n", "a\n\t\tb\nc\n"},
		{"\n\t\t\ta\n\t\tb\n\t\t\t\n\t\t\t\tc\n\t\t", "a\nb\n\n\tc\n"},
	} {
		actual := Dedent(tr.a)
		if actual != tr.b {
			t.Errorf("Dedent(%q) != %q: got %q", tr.a, tr.b, actual)
		}
	}
}
