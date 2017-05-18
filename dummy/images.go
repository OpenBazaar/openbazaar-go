package main

import (
	"encoding/base64"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/OpenBazaar/openbazaar-go/core"
	"github.com/icrowley/fake"
)

const randomImageURL = "http://lorempixel.com/600/400/"

var imageHTTPClient = &http.Client{
	Timeout: time.Second * 10,
}

type randomImage struct {
	filename string
	*core.Images
}

func newRandomImage(node *core.OpenBazaarNode) (*randomImage, error) {
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
	img := &randomImage{filename: fake.Word()}
	img.Images, err = node.SetProductImages(base64Img, img.filename)
	if err != nil {
		return nil, err
	}

	return img, nil
}
