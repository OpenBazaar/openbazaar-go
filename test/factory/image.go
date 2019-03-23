package factory

import "github.com/OpenBazaar/openbazaar-go/pb"

func NewImage() *pb.Listing_Item_Image {
	return &pb.Listing_Item_Image{
		Filename: "image.jpg",
		Tiny:     "QmUy4jh5mGNZvLkjies1RWM4YuvJh5o2FYopNPVYwrRVGV",
		Small:    "QmUy4jh5mGNZvLkjies1RWM4YuvJh5o2FYopNPVYwrRVGV",
		Medium:   "QmUy4jh5mGNZvLkjies1RWM4YuvJh5o2FYopNPVYwrRVGV",
		Large:    "QmUy4jh5mGNZvLkjies1RWM4YuvJh5o2FYopNPVYwrRVGV",
		Original: "QmUy4jh5mGNZvLkjies1RWM4YuvJh5o2FYopNPVYwrRVGV",
	}
}
