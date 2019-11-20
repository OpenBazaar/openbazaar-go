package factory

import (
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/openbazaar-go/repo"
)

func NewProfile() *repo.Profile {
	return &repo.Profile{
		Moderator: true,
		ModeratorInfo: &repo.ModeratorInfo{
			Fee: &repo.ModeratorFee{
				FeeType: pb.Moderator_Fee_FIXED_PLUS_PERCENTAGE.String(),
				FixedFee: &repo.ModeratorFixedFee{
					Amount:         "1234",
					AmountCurrency: NewCurrencyDefinition("BTC"),
				},
				Percentage: 1.1, // represents 0.011%
			},
		},
	}
}

func NewProfileProtobuf() *pb.Profile {
	return &pb.Profile{
		Moderator: true,
		ModeratorInfo: &pb.Moderator{
			Fee: &pb.Moderator_Fee{
				FixedFee: &pb.Moderator_Price{
					BigAmount: "1234",
					AmountCurrency: &pb.CurrencyDefinition{
						Code:         "BTC",
						Divisibility: 8,
					},
				},
				Percentage: 1.1,
				FeeType:    pb.Moderator_Fee_FIXED_PLUS_PERCENTAGE,
			},
		},
	}
}
