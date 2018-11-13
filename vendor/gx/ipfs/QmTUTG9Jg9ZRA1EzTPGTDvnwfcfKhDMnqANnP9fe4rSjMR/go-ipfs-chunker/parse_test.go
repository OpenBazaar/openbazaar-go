package chunk

import (
	"bytes"
	"testing"
)

func TestParseRabin(t *testing.T) {
	max := 1000
	r := bytes.NewReader(randBuf(t, max))
	chk1 := "rabin-18-25-32"
	chk2 := "rabin-15-23-31"
	_, err := parseRabinString(r, chk1)
	if err != nil {
		t.Errorf(err.Error())
	}
	_, err = parseRabinString(r, chk2)
	if err == ErrRabinMin {
		t.Log("it should be ErrRabinMin here.")
	}
}

func TestParseSize(t *testing.T) {
	max := 1000
	r := bytes.NewReader(randBuf(t, max))
	size1 := "size-0"
	size2 := "size-32"
	_, err := FromString(r, size1)
	if err == ErrSize {
		t.Log("it should be ErrSize here.")
	}
	_, err = FromString(r, size2)
	if err == ErrSize {
		t.Fatal(err)
	}
}
