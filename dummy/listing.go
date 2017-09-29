package main

import (
	"math/rand"
	"strconv"
	"time"

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

func newRandomListing(randomImages chan (*pb.Profile_Image)) *pb.Listing {
	title := fake.ProductName()
	slug := slugify(title)
	// sku := slug + "-" + fake.Digits()

	tags := make([]string, rand.Intn(9)+1)
	tags[0] = fake.Word()
	for i := 1; i < len(tags); i++ {
		tags[i] = fake.Word()
	}

	// Create a random amount of options with random amounts of varients
	options := make([]*pb.Listing_Item_Option, rand.Intn(4)+2)
	variantCount := rand.Intn(2) + 2

	for i := 0; i < len(options); i++ {
		options[i] = &pb.Listing_Item_Option{
			Name:        fake.ProductName(),
			Description: fake.Sentence(),
			Variants:    make([]*pb.Listing_Item_Option_Variant, variantCount),
		}

		// Ensure description is <= 70
		if len(options[i].Description) > 70 {
			options[i].Description = options[i].Description[:70]
		}

		// Set variant name and image
		for j := 0; j < len(options[i].Variants); j++ {
			// Get fake name <= 40 characters
			name := fake.Word()
			if len(name) > 40 {
				name = name[:40]
			}

			// Create variant
			options[i].Variants[j] = &pb.Listing_Item_Option_Variant{Name: name}

			// Get fake image
			timeout := time.NewTimer(2 * time.Second)
			select {
			case <-timeout.C:
				break
			case image := <-randomImages:
				options[i].Variants[j].Image = &pb.Listing_Item_Image{
					Filename: fake.Word(),
					Tiny:     image.Tiny,
					Small:    image.Small,
					Medium:   image.Medium,
					Large:    image.Large,
					Original: image.Original,
				}
			}
		}
	}

	countries := make([]pb.CountryCode, rand.Intn(254)+1)
	for i := 0; i < len(countries); i++ {
		countries[i] = pb.CountryCode(rand.Intn(255))
	}

	imageCount := rand.Intn(6) + 1
	images := make([]*pb.Listing_Item_Image, 0, imageCount)
	stopGettingImages := make(chan (struct{}))
	time.AfterFunc(10*time.Second, func() {
		close(stopGettingImages)
	})

IMAGE_LOOP:
	for i := 0; i < imageCount; i++ {
		select {
		case <-stopGettingImages:
			break IMAGE_LOOP
		case image := <-randomImages:
			images = append(images, &pb.Listing_Item_Image{
				Filename: fake.Word(),
				Tiny:     image.Tiny,
				Small:    image.Small,
				Medium:   image.Medium,
				Large:    image.Large,
				Original: image.Original,
			})
		}
	}

	return &pb.Listing{
		Slug: slug,
		Metadata: &pb.Listing_Metadata{
			Version:            1,
			AcceptedCurrencies: []string{"btc"},
			PricingCurrency:    "btc",
			Expiry:             &timestamp.Timestamp{Seconds: 2147483647},
			Format:             pb.Listing_Metadata_FIXED_PRICE,
			ContractType:       pb.Listing_Metadata_ContractType(uint32(rand.Intn(3))),
		},
		Item: &pb.Listing_Item{
			// Skus:           sku,
			Title:          title,
			Tags:           tags,
			Options:        options,
			Nsfw:           isNSFW(),
			Description:    fake.Paragraphs(),
			Price:          uint64(rand.Intn(99)) + 50,
			ProcessingTime: strconv.Itoa(rand.Intn(14)+1) + " days",
			Categories:     []string{categories[rand.Intn(len(categories))]},
			Grams:          float32(rand.Intn(5000)),
			Condition:      conditions[rand.Intn(len(conditions))],
			Images:         images,
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
							MaxRange: 999,
							Price:    uint64(rand.Intn(999)),
						},
					},
				},
			},
		},

		TermsAndConditions: fake.Sentences(),
		RefundPolicy:       fake.Sentence(),
	}
}
