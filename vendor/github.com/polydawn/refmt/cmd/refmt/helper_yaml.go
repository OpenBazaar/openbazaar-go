package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"

	"github.com/go-yaml/yaml"
	"github.com/polydawn/refmt/obj"
	"github.com/polydawn/refmt/obj/atlas"
	"github.com/polydawn/refmt/shared"
	"github.com/polydawn/refmt/tok"
)

/*
	Turn yaml into a TokenSource... by running it through a third-party
	parser which emits objects, then applying our obj.NewMarshaller to get
	tokens again.

	Obviously, this does not function streamingly.  (Not only does the full
	object in the middle make a blocking point, the third-party library
	itself does not operate streamingly on the bytes, either.)
*/
func newYamlTokenSource(in io.Reader) shared.TokenSource {
	byts, err := ioutil.ReadAll(in)
	if err != nil {
		return errthunkTokenSource{fmt.Errorf("refmt: error reading: %s", err)}
	}
	byts = tab2space(byts)
	var barf interface{}
	if err := yaml.Unmarshal(byts, &barf); err != nil {
		return errthunkTokenSource{fmt.Errorf("refmt: error deserializing yaml: %s", err)}
	}
	barf = stringifyMapKeys(barf)
	tokenSrc := obj.NewMarshaller(atlas.MustBuild())
	if err := tokenSrc.Bind(barf); err != nil {
		return errthunkTokenSource{fmt.Errorf("refmt: error deserializing yaml: %s", err)}
	}
	return tokenSrc
}

type errthunkTokenSource struct {
	err error
}

func (x errthunkTokenSource) Step(*tok.Token) (done bool, err error) {
	return true, x.err
}

/*
	Yaml things anything can be a map key.
	Most things think only strings can be a map key.
	This func makes yaml outputs into what everyone else expects.
*/
func stringifyMapKeys(value interface{}) interface{} {
	switch value := value.(type) {
	case map[interface{}]interface{}:
		next := make(map[string]interface{}, len(value))
		for k, v := range value {
			next[k.(string)] = stringifyMapKeys(v)
		}
		return next
	case []interface{}:
		for i := 0; i < len(value); i++ {
			value[i] = stringifyMapKeys(value[i])
		}
		return value
	default:
		return value
	}
}

/*
	Okay so *I* think tabs are cool and really not that hard to deal with
	and so our yaml handling will accept tabs.

	... By converting them shamelessly to two-space pairs, because that's
	what the 3rd-party yaml parser library we're leaning on is hung up on.
	(No, I'm not writing a yaml parser.  Yaml is insane.  Nope.)
*/
func tab2space(x []byte) []byte {
	// flip into lines, replace leading tabs with spaces, flip back to bytes, cry at the loss of spilt cycles
	// fortunately it's all ascii transforms, so at least we don't have to convert to strings and back
	// unfortunately it's an expansion (yaml needs at least two spaces of indentation) so yep reallocations / large memmoves become unavoidable
	lines := bytes.Split(x, []byte{'\n'})
	buf := bytes.Buffer{}
	for i, line := range lines {
		for n := range line {
			if line[n] != '\t' {
				buf.Write(line[n:])
				break
			}
			buf.Write([]byte{' ', ' '})
		}
		if i != len(lines)-1 {
			buf.WriteByte('\n')
		}
	}
	return buf.Bytes()
}
