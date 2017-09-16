package api

import (
	"bytes"
	"encoding/json"
	"github.com/OpenBazaar/jsonpb"
	"github.com/golang/protobuf/proto"
	"github.com/microcosm-cc/bluemonday"
	"strings"
)

var sanitizer *bluemonday.Policy

func init() {
	sanitizer = bluemonday.UGCPolicy()
}

func SanitizeJSON(s []byte) ([]byte, error) {
	d := json.NewDecoder(bytes.NewReader(s))
	d.UseNumber()

	var i interface{}
	err := d.Decode(&i)
	if err != nil {
		return nil, err
	}
	sanitize(i)

	i = bytes.Replace(i, []byte("&amp;"), []byte("&"), -1)

	return json.MarshalIndent(i, "", "    ")
}

func SanitizeProtobuf(jsonEncodedProtobuf string, m proto.Message) ([]byte, error) {
	ret, err := SanitizeJSON([]byte(jsonEncodedProtobuf))
	if err != nil {
		return nil, err
	}
	err = jsonpb.UnmarshalString(string(ret), m)
	if err != nil {
		return nil, err
	}
	marshaler := jsonpb.Marshaler{
		EnumsAsInts:  false,
		EmitDefaults: true,
		Indent:       "    ",
		OrigName:     false,
	}
	out, err := marshaler.MarshalToString(m)
	if err != nil {
		return nil, err
	}
	out = strings.Replace(out, "&amp;", "&", -1)
	return []byte(out), nil
}

func sanitize(data interface{}) {
	switch d := data.(type) {
	case map[string]interface{}:
		for k, v := range d {
			switch v.(type) {
			case string:
				d[k] = sanitizer.Sanitize(v.(string))
			case map[string]interface{}:
				sanitize(v)
			case []interface{}:
				sanitize(v)
			case nil:
				delete(d, k)
			}
		}
	case []interface{}:
		if len(d) > 0 {
			switch d[0].(type) {
			case string:
				for i, s := range d {
					d[i] = sanitizer.Sanitize(s.(string))
				}
			case map[string]interface{}:
				for _, t := range d {
					sanitize(t)
				}
			case []interface{}:
				for _, t := range d {
					sanitize(t)
				}
			}
		}
	}
}
