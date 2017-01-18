package main

import (
	"math/rand"
	"strconv"

	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/icrowley/fake"
)

var conditions = []string{"New", "Used", "Bad", "Most Excellent"}

var categories = []string{
	"Arts",
	"Electronics",
	"Entertainment",
	"Home",
	"Food",
	"Personal",
	"Services",
	"Digital Goods",
	"Other",
}

func newRandomListing() *pb.ListingReqApi {
	title := fake.ProductName()
	slug := slugify(title)
	sku := slug + "-" + fake.Digits()

	tags := make([]string, rand.Intn(9)+1)
	tags[0] = fake.Word()
	for i := 1; i < len(tags); i++ {
		tags[i] = fake.Word()
	}

	// Create a random amount of options with random amounts of varients
	options := make([]*pb.Listing_Item_Option, rand.Intn(4)+2)
	for i := 0; i < len(options); i++ {
		options[i] = &pb.Listing_Item_Option{
			Name:        fake.ProductName(),
			Description: fake.Sentence(),
			Variants:    make([]*pb.Listing_Item_Option_Variants, rand.Intn(2)+2),
		}

		// Ensure description is <= 70
		if len(options[i].Description) > 70 {
			options[i].Description = options[i].Description[:70]
		}

		for j := 0; j < len(options[i].Variants); j++ {
			options[i].Variants[j] = &pb.Listing_Item_Option_Variants{
				Name: title + " " + fake.ProductName(),
				Image: &pb.Listing_Item_Image{
					Filename: "example.jpg",
					Original: "QmdfiTnhj1oqiCDmhxu1gdgW6ZqtR7D6ZE7j7CqWUHgKJ8",
					Tiny:     "QmbKPEBbzVwax8rnrdxLepfmNkdTqnw2RSfJE19iax3fLK",
					Small:    "QmQ77aAsYjs1rxcp7xZ5qUki1RGDGjQ99cok3ynUMF8Sc5",
					Medium:   "QmVoh493xbSaKYV9yLtEapaGQ7J31gdCiDQHGSd86PNo8B",
					Large:    "QmUWuTZhjUuY8VYWVZrcodsrsbQvckSZwTRxmJ3Avhnw1y",
				},
				PriceModifier: int64(rand.Intn(50)),
			}

			// Ensure name is <= 40 chars
			if len(options[i].Variants[j].Name) > 40 {
				options[i].Variants[j].Name = options[i].Variants[j].Name[:40]
			}
		}
	}

	countries := make([]pb.CountryCode, rand.Intn(254)+1)
	for i := 0; i < len(countries); i++ {
		countries[i] = pb.CountryCode(rand.Intn(255))
	}

	return &pb.ListingReqApi{
		Listing: &pb.Listing{
			Slug: slug,
			Metadata: &pb.Listing_Metadata{
				Version:          1,
				AcceptedCurrency: "btc",
				PricingCurrency:  "btc",
				Expiry:           &timestamp.Timestamp{Seconds: 2147483647},
				Format:           pb.Listing_Metadata_FIXED_PRICE,
				ContractType:     pb.Listing_Metadata_ContractType(uint32(rand.Intn(3))),
			},
			Item: &pb.Listing_Item{
				Sku:            sku,
				Title:          title,
				Tags:           tags,
				Options:        options,
				Nsfw:           isNSFW(),
				Description:    fake.Paragraphs(),
				Price:          uint64(rand.Intn(9999)),
				ProcessingTime: strconv.Itoa(rand.Intn(14)+1) + " days",
				Categories:     []string{categories[rand.Intn(len(categories))]},
				Grams:          float32(rand.Intn(5000)),
				Condition:      conditions[rand.Intn(len(conditions))],
				Images: []*pb.Listing_Item_Image{
					{
						Filename: "example.jpg",
						Original: "QmdfiTnhj1oqiCDmhxu1gdgW6ZqtR7D6ZE7j7CqWUHgKJ8",
						Tiny:     "QmbKPEBbzVwax8rnrdxLepfmNkdTqnw2RSfJE19iax3fLK",
						Small:    "QmQ77aAsYjs1rxcp7xZ5qUki1RGDGjQ99cok3ynUMF8Sc5",
						Medium:   "QmVoh493xbSaKYV9yLtEapaGQ7J31gdCiDQHGSd86PNo8B",
						Large:    "QmUWuTZhjUuY8VYWVZrcodsrsbQvckSZwTRxmJ3Avhnw1y",
					},
				},
			},
			ShippingOptions: []*pb.Listing_ShippingOption{
				{
					Name:    "usps",
					Type:    pb.Listing_ShippingOption_FIXED_PRICE,
					Regions: countries,
					Services: []*pb.Listing_ShippingOption_Service{
						{
							Name:              "standard",
							Price:             uint64(rand.Intn(999)),
							EstimatedDelivery: fake.Digits() + " days",
						},
					},
					ShippingRules: &pb.Listing_ShippingOption_ShippingRules{
						RuleType: pb.Listing_ShippingOption_ShippingRules_FLAT_FEE_QUANTITY_RANGE,
						Rules: []*pb.Listing_ShippingOption_ShippingRules_Rule{
							{
								MinRange: 0,
								MaxRange: 99999999,
								Price:    uint64(rand.Intn(999)),
							},
						},
					},
				},
			},

			TermsAndConditions: fake.Sentences(),
			RefundPolicy:       fake.Sentence(),
		},
	}
}
