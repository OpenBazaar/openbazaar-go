package factory

import "github.com/OpenBazaar/openbazaar-go/pb"

func NewImage() *pb.Listing_Item_Image {
	return &pb.Listing_Item_Image{
		Filename: "image.jpg",
		Tiny:     "zb2rhjqhgN4Pv1SJFNpCQMjv2h8PQEGqAioMhkZjkKDyPW5E2",
		Small:    "zb2rhjqhgN4Pv1SJFNpCQMjv2h8PQEGqAioMhkZjkKDyPW5E2",
		Medium:   "zb2rhjqhgN4Pv1SJFNpCQMjv2h8PQEGqAioMhkZjkKDyPW5E2",
		Large:    "zb2rhjqhgN4Pv1SJFNpCQMjv2h8PQEGqAioMhkZjkKDyPW5E2",
		Original: "zb2rhjqhgN4Pv1SJFNpCQMjv2h8PQEGqAioMhkZjkKDyPW5E2",
	}
}
