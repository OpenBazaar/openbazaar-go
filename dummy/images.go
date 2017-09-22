package main

import (
	"encoding/base64"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/OpenBazaar/openbazaar-go/core"
	"github.com/icrowley/fake"
	"github.com/OpenBazaar/openbazaar-go/pb"
)

const randomImageURL = "http://lorempixel.com/600/400/"

var imageHTTPClient = &http.Client{
	Timeout: time.Second * 10,
}

func newRandomImage(node *core.OpenBazaarNode) (*pb.Profile_Image, error) {
	// Get random image
	resp, err := imageHTTPClient.Get(randomImageURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Get base64 encoded image
	buf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	base64Img := base64.StdEncoding.EncodeToString(buf)

	// Decode image and save
	img, err := node.SetProductImages(base64Img, fake.Word())
	if err != nil {
		return nil, err
	}
	return img, nil
}
