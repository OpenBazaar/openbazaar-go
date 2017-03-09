package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/microcosm-cc/bluemonday"
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
	if err := sanitize(i); err != nil {
		return nil, err
	}
	return json.MarshalIndent(i, "", "    ")
}

func sanitize(data interface{}) error {
	switch d := data.(type) {
	case map[string]interface{}:
		for k, v := range d {
			switch v.(type) {
			case string:
				d[k] = sanitizer.Sanitize(v.(string))
			case map[string]interface{}:
				return sanitize(v)
			case []interface{}:
				return sanitize(v)
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
					return sanitize(t)
				}
			case []interface{}:
				for _, t := range d {
					return sanitize(t)
				}
			}
		}
	default:
		return fmt.Errorf("Unsupported type: %t", d)
	}
	return nil
}
