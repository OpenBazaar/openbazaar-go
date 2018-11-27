package main

import (
	"fmt"
	"testing"

	c "gx/ipfs/QmPSQnBKM9g7BaUcZCvswUJVscQ1ipjmwxN5PXCjkp9EQ7/go-cid"
)

func TestCidConv(t *testing.T) {
	cidv0 := "QmUNLLsPACCz1vLxQVkXqqLX5R1X345qqfHbsf67hvA3Nn"
	cidv1 := "zdj7WbTaiJT1fgatdet9Ei9iDB5hdCxkbVyhyh8YTUnXMiwYi"
	cid, err := c.Decode(cidv0)
	if err != nil {
		t.Fatal(err)
	}
	cid, err = toCidV1(cid)
	if err != nil {
		t.Fatal(err)
	}
	if cid.String() != cidv1 {
		t.Fatal("conversion failure")
	}
	cid, err = toCidV0(cid)
	if err != nil {
		t.Fatal(err)
	}
	cidStr := cid.String()
	if cidStr != cidv0 {
		t.Error(fmt.Sprintf("conversion failure, expected: %s; but got: %s", cidv0, cidStr))
	}
}

func TestBadCidConv(t *testing.T) {
	// this cid is a raw leaf and should not be able to convert to cidv0
	cidv1 := "zb2rhhzX7uSKrtQ2ZZXFAabKiKFYZrJqKY2KE1cJ8yre2GSWZ"
	cid, err := c.Decode(cidv1)
	if err != nil {
		t.Fatal(err)
	}
	cid, err = toCidV0(cid)
	if err == nil {
		t.Fatal("expected failure")
	}
}
