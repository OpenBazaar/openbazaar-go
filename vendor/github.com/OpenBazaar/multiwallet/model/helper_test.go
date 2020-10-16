package model_test

import (
	"testing"

	"github.com/OpenBazaar/multiwallet/model"
)

func Test_toFloat64(t *testing.T) {
	f, err := model.ToFloat(12.345)
	if err != nil {
		t.Error(err)
	}
	if f != 12.345 {
		t.Error("Returned incorrect float")
	}
	f, err = model.ToFloat("456.789")
	if err != nil {
		t.Error(err)
	}
	if f != 456.789 {
		t.Error("Returned incorrect float")
	}
}
