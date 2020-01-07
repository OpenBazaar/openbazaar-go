package repo

import (
	"github.com/OpenBazaar/jsonpb"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"testing"
)

func TestToV5Order(t *testing.T) {
	m := jsonpb.Marshaler{
		Indent: "",
	}

	tests := []struct {
		name           string
		oldFormatOrder *pb.Order
		expected       string
	}{
		{
			name: "refund fee test",
			oldFormatOrder: &pb.Order{
				RefundFee: 1000,
				Payment:   &pb.Order_Payment{},
			},
			expected: `{"payment":{"method":"ADDRESS_REQUEST"},"bigRefundFee":"1000"}`,
		},
		{
			name: "item quantity test",
			oldFormatOrder: &pb.Order{
				Items: []*pb.Order_Item{
					{
						Quantity: 19,
					},
				},
				Payment: &pb.Order_Payment{},
			},
			expected: `{"items":[{"bigQuantity":"19"}],"payment":{"method":"ADDRESS_REQUEST"}}`,
		},
		{
			name: "item quantity64 test",
			oldFormatOrder: &pb.Order{
				Items: []*pb.Order_Item{
					{
						Quantity64: 19,
					},
				},
				Payment: &pb.Order_Payment{},
			},
			expected: `{"items":[{"bigQuantity":"19"}],"payment":{"method":"ADDRESS_REQUEST"}}`,
		},
		{
			name: "payment amount test",
			oldFormatOrder: &pb.Order{
				Payment: &pb.Order_Payment{
					Amount: 2000,
				},
			},
			expected: `{"payment":{"method":"ADDRESS_REQUEST","bigAmount":"2000"}}`,
		},
		{
			name: "payment amount currency",
			oldFormatOrder: &pb.Order{
				Payment: &pb.Order_Payment{
					Coin: "BTC",
				},
			},
			expected: `{"payment":{"method":"ADDRESS_REQUEST","amountCurrency":{"code":"BTC","divisibility":8}}}`,
		},
	}

	for _, test := range tests {
		order, err := ToV5Order(test.oldFormatOrder, MainnetCurrencies().Lookup)
		if err != nil {
			t.Errorf("Test %s conversion failed: %s", test.name, err)
		}

		out, err := m.MarshalToString(order)
		if err != nil {
			t.Errorf("Test %s marshalling failed: %s", test.name, err)
		}

		if out != test.expected {
			t.Errorf("Test %s incorrect output: Expected %s, got %s", test.name, test.expected, out)
		}
	}
}
