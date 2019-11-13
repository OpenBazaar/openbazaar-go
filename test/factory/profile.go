package factory

import (
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/openbazaar-go/repo"
)

func NewProfile() *repo.Profile {
	return &repo.Profile{
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
