package repo

import (
	"errors"
	"fmt"
	"time"
)

var ErrUnknownAPITimeFormat = errors.New("unknown api time format")

const JSONAPITimeFormat = `"2006-01-02T15:04:05.999999999Z07:00"` // time.RFC3339Nano

type APITime time.Time

func (t APITime) MarshalJSON() ([]byte, error) {
	return []byte(time.Time(t).Format(JSONAPITimeFormat)), nil
}

func (t *APITime) UnmarshalJSON(b []byte) error {
	if value, err := time.Parse(q(time.RFC3339), string(b)); err == nil {
		*t = APITime(value)
		return nil
	}
	return ErrUnknownAPITimeFormat
}

func q(format string) string {
	return fmt.Sprintf(`"%s"`, format)
}

func (t APITime) String() string {
	return time.Time(t).String()
}
