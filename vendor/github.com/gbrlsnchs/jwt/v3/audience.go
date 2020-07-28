package jwt

import "encoding/json"

// Audience is a special claim that may either be
// a single string or an array of strings, as per the RFC 7519.
type Audience []string

// MarshalJSON implements a marshaling function for "aud" claim.
func (a Audience) MarshalJSON() ([]byte, error) {
	switch len(a) {
	case 0:
		return json.Marshal("") // nil or empty slice returns an empty string
	case 1:
		return json.Marshal(a[0])
	default:
		return json.Marshal([]string(a))
	}
}

// UnmarshalJSON implements an unmarshaling function for "aud" claim.
func (a *Audience) UnmarshalJSON(b []byte) error {
	var (
		v   interface{}
		err error
	)
	if err = json.Unmarshal(b, &v); err != nil {
		return err
	}
	switch vv := v.(type) {
	case string:
		aud := make(Audience, 1)
		aud[0] = vv
		*a = aud
	case []interface{}:
		aud := make(Audience, len(vv))
		for i := range vv {
			aud[i] = vv[i].(string)
		}
		*a = aud
	}
	return nil
}
