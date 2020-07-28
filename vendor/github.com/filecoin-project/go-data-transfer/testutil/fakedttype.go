package testutil

import (
	"testing"

	"github.com/stretchr/testify/require"

	datatransfer "github.com/filecoin-project/go-data-transfer"
	"github.com/filecoin-project/go-data-transfer/encoding"
	"github.com/filecoin-project/go-data-transfer/message"
)

//go:generate cbor-gen-for FakeDTType

// FakeDTType simple fake type for using with registries
type FakeDTType struct {
	Data string
}

// Type satisfies registry.Entry
func (ft FakeDTType) Type() datatransfer.TypeIdentifier {
	return "FakeDTType"
}

// AssertFakeDTVoucher asserts that a data transfer requests contains the expected fake data transfer voucher type
func AssertFakeDTVoucher(t *testing.T, request message.DataTransferRequest, expected *FakeDTType) {
	require.Equal(t, datatransfer.TypeIdentifier("FakeDTType"), request.VoucherType())
	fakeDTDecoder, err := encoding.NewDecoder(&FakeDTType{})
	require.NoError(t, err)
	decoded, err := request.Voucher(fakeDTDecoder)
	require.NoError(t, err)
	require.Equal(t, expected, decoded)
}

// AssertEqualFakeDTVoucher asserts that two requests have the same fake data transfer voucher
func AssertEqualFakeDTVoucher(t *testing.T, expectedRequest message.DataTransferRequest, request message.DataTransferRequest) {
	require.Equal(t, expectedRequest.VoucherType(), request.VoucherType())
	fakeDTDecoder, err := encoding.NewDecoder(&FakeDTType{})
	require.NoError(t, err)
	expectedDecoded, err := request.Voucher(fakeDTDecoder)
	require.NoError(t, err)
	decoded, err := request.Voucher(fakeDTDecoder)
	require.NoError(t, err)
	require.Equal(t, expectedDecoded, decoded)
}

// NewFakeDTType returns a fake dt type with random data
func NewFakeDTType() *FakeDTType {
	return &FakeDTType{Data: string(RandomBytes(100))}
}

var _ datatransfer.Registerable = &FakeDTType{}
