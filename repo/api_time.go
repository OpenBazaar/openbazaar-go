package repo

import (
	"errors"
	"fmt"
	"time"
)

var ErrUnknownAPITimeFormat = errors.New("unknown api time format")

const JSONAPITimeFormat = `"2006-01-02T15:04:05.999999999Z07:00"` // time.RFC3339Nano

type APITime struct {
	time.Time
}

// NewAPITime returns a pointer to a new APITime instance
func NewAPITime(t time.Time) *APITime {
	var val = APITime{t}
	return &val
}

func (t APITime) MarshalJSON() ([]byte, error) {
	return []byte(t.Time.Format(JSONAPITimeFormat)), nil
}

func (t *APITime) UnmarshalJSON(b []byte) error {
	if value, err := time.Parse(q(time.RFC3339), string(b)); err == nil {
		*t = APITime{value}
		return nil
	}
	return ErrUnknownAPITimeFormat
}

func q(format string) string {
	return fmt.Sprintf(`"%s"`, format)
}

func (t APITime) String() string {
	return t.Time.String()
}
