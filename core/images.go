package core

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"gx/ipfs/QmTbxNB1NwDesLmKTscr4udL2tVP7MaxvXnD1D9yX7g3PN/go-cid"
	"image" // load gif
	"image/color/palette"
	"image/draw"
	"image/gif"
	_ "image/gif" // load png
	"image/jpeg"
	_ "image/png"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	netUrl "net/url"
	"os"
	"path"
	"strings"
	"time"

	ipath "gx/ipfs/QmQAgv6Gaoe2tQpcabqwKXKChp2MZ7i3UXv9DqTTaxCaTR/go-path"
	cid "gx/ipfs/QmTbxNB1NwDesLmKTscr4udL2tVP7MaxvXnD1D9yX7g3PN/go-cid"

	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/discordapp/lilliput"
	"github.com/nfnt/resize"
)

// SetAvatarImages - set avatar image from the base64 encoded image string
func (n *OpenBazaarNode) SetAvatarImages(base64ImageData string) (*pb.Profile_Image, error) {
	imageHashes, err := n.resizeImage(base64ImageData, "avatar", 60, 60)
	if err != nil {
		return nil, err
	}

	profile, err := n.GetProfile()
	if err != nil {
		return nil, err
	}

	profile.AvatarHashes = imageHashes
	err = n.UpdateProfile(&profile)
	if err != nil {
		return nil, err
	}
	return imageHashes, nil
}

// SetHeaderImages - set header image from the base64 encoded string
func (n *OpenBazaarNode) SetHeaderImages(base64ImageData string) (*pb.Profile_Image, error) {
	imageHashes, err := n.resizeImage(base64ImageData, "header", 315, 90)
	if err != nil {
		return nil, err
	}

	profile, err := n.GetProfile()
	if err != nil {
		return nil, err
	}

	profile.HeaderHashes = imageHashes
	err = n.UpdateProfile(&profile)
	if err != nil {
		return nil, err
	}
	return imageHashes, nil
}

// SetProductImages - use the original image in a base64 string format and generate tiny,
// small, medium and large images for the product
func (n *OpenBazaarNode) SetProductImages(base64ImageData, filename string) (*pb.Profile_Image, error) {
	return n.resizeImage(base64ImageData, filename, 120, 120)
}

func base64ToReader(base64ImageData string) io.Reader {
	return base64.NewDecoder(base64.StdEncoding, strings.NewReader(base64ImageData))
}

func (n *OpenBazaarNode) resizeImage(base64ImageData, filename string, baseWidth, baseHeight uint) (*pb.Profile_Image, error) {
	imgPath := path.Join(n.RepoPath, "root", "images")

	imgType, err := decodeImageType(base64ImageData)
	if err != nil {
		return nil, err
	}

	if imgType == "gif" {
		reader := base64.NewDecoder(base64.StdEncoding, strings.NewReader(base64ImageData))
		imgGif, err := gif.DecodeAll(reader)
		if err != nil {
			return nil, err
		}
		reader = base64.NewDecoder(base64.StdEncoding, strings.NewReader(base64ImageData))
		imgCfg, err := gif.DecodeConfig(reader)
		if err != nil {
			return nil, err
		}

		t, err := n.addResizedGif(base64ImageData, &imgCfg, 1*baseWidth, 1*baseHeight, path.Join(imgPath, "tiny", filename), imgType)
		if err != nil {
			return nil, err
		}
		s, err := n.addResizedGif(base64ImageData, &imgCfg, 2*baseWidth, 2*baseHeight, path.Join(imgPath, "small", filename), imgType)
		if err != nil {
			return nil, err
		}
		m, err := n.addResizedGif(base64ImageData, &imgCfg, 4*baseWidth, 4*baseHeight, path.Join(imgPath, "medium", filename), imgType)
		if err != nil {
			return nil, err
		}
		l, err := n.addResizedGif(base64ImageData, &imgCfg, 8*baseWidth, 8*baseHeight, path.Join(imgPath, "large", filename), imgType)
		if err != nil {
			return nil, err
		}

		// Add original file
		out, err := os.Create(path.Join(imgPath, "original", filename))
		if err != nil {
			return nil, err
		}

		// Write to file
		err = gif.EncodeAll(out, imgGif)
		if err != nil {
			return nil, err
		}

		out.Close()
		if err != nil {
			return nil, err
		}
		o, err := ipfs.AddFile(n.IpfsNode, path.Join(imgPath, "original", filename))
		if err != nil {
			return nil, err
		}
		return &pb.Profile_Image{Tiny: t, Small: s, Medium: m, Large: l, Original: o}, nil
	} else {
		img, imgCfg, err := decodeImageData(base64ImageData)
		if err != nil {
			return nil, err
		}

		t, err := n.addResizedImage(img, imgCfg, 1*baseWidth, 1*baseHeight, path.Join(imgPath, "tiny", filename), imgType)
		if err != nil {
			return nil, err
		}
		s, err := n.addResizedImage(img, imgCfg, 2*baseWidth, 2*baseHeight, path.Join(imgPath, "small", filename), imgType)
		if err != nil {
			return nil, err
		}
		m, err := n.addResizedImage(img, imgCfg, 4*baseWidth, 4*baseHeight, path.Join(imgPath, "medium", filename), imgType)
		if err != nil {
			return nil, err
		}
		l, err := n.addResizedImage(img, imgCfg, 8*baseWidth, 8*baseHeight, path.Join(imgPath, "large", filename), imgType)
		if err != nil {
			return nil, err
		}
		o, err := n.addImage(img, path.Join(imgPath, "original", filename), imgType)
		if err != nil {
			return nil, err
		}
		return &pb.Profile_Image{Tiny: t, Small: s, Medium: m, Large: l, Original: o}, nil
	}
	return nil, err
}

func (n *OpenBazaarNode) addImage(img image.Image, imgPath string, imgType string) (string, error) {
	out, err := os.Create(imgPath)
	if err != nil {
		return "", err
	}
	err = jpeg.Encode(out, img, nil)
	if err != nil {
		return "", err
	}
	out.Close()
	if err != nil {
		return "", err
	}
	return ipfs.AddFile(n.IpfsNode, imgPath)
}

func ImageToPaletted(img image.Image) *image.Paletted {
	b := img.Bounds()
	pm := image.NewPaletted(b, palette.Plan9)
	draw.FloydSteinberg.Draw(pm, b, img, image.ZP)
	return pm
}

func ProcessImage(img image.Image) image.Image {
	return resize.Resize(250, 0, img, resize.NearestNeighbor)
}

func (n *OpenBazaarNode) addResizedGif(base64data string, imgCfg *image.Config, w, h uint, imgPath string, imgType string) (string, error) {

	width, height := getImageAttributes(w, h, uint(imgCfg.Width), uint(imgCfg.Height))

	inputBuf, _ := base64.StdEncoding.DecodeString(base64data)
	decoder, err := lilliput.NewDecoder(inputBuf)
	// this error reflects very basic checks,
	// mostly just for the magic bytes of the file to match known image formats
	if err != nil {
		fmt.Printf("error decoding image, %s\n", err)
		os.Exit(1)
	}
	defer decoder.Close()

	// get ready to resize image,
	// using 8192x8192 maximum resize buffer size
	ops := lilliput.NewImageOps(8192)
	defer ops.Close()

	// create a buffer to store the output image, 50MB in this case
	outputImg := make([]byte, 50*1024*1024)

	opts := &lilliput.ImageOptions{
		FileType:     ".gif",
		Width:        int(width),
		Height:       int(height),
		ResizeMethod: lilliput.ImageOpsResize,
	}

	// resize and transcode image
	outputImg, err = ops.Transform(decoder, opts, outputImg)
	if err != nil {
		fmt.Printf("error transforming image, %s\n", err)
		os.Exit(1)
	}

	out, err := os.Create(imgPath)
	if err != nil {
		return "", err
	}

	err = ioutil.WriteFile(imgPath, outputImg, 0400)
	if err != nil {
		fmt.Printf("error writing out resized image, %s\n", err)
		os.Exit(1)
	}

	out.Close()
	return ipfs.AddFile(n.IpfsNode, imgPath)
}

func (n *OpenBazaarNode) addResizedImage(img image.Image, imgCfg *image.Config, w, h uint, imgPath string, imgType string) (string, error) {
	width, height := getImageAttributes(w, h, uint(imgCfg.Width), uint(imgCfg.Height))
	newImg := resize.Resize(width, height, img, resize.Lanczos3)
	return n.addImage(newImg, imgPath, imgType)
}

func decodeImageData(base64ImageData string) (image.Image, error) {
	reader := base64.NewDecoder(base64.StdEncoding, strings.NewReader(base64ImageData))
	img, err := imaging.Decode(reader, imaging.AutoOrientation(true))
	if err != nil {
		return nil, err
	}
	return img, err
}

func getImageAttributes(targetWidth, targetHeight, imgWidth, imgHeight int) (width, height int) {
	targetRatio := float32(targetWidth) / float32(targetHeight)
	imageRatio := float32(imgWidth) / float32(imgHeight)
	var h, w float32
	if imageRatio > targetRatio {
		h = float32(targetHeight)
		w = float32(targetHeight) * imageRatio
	} else {
		w = float32(targetWidth)
		h = float32(targetWidth) * (float32(imgHeight) / float32(imgWidth))
	}
	return int(w), int(h)
}

// FetchAvatar - fetch image avatar from ipfs
func (n *OpenBazaarNode) FetchAvatar(peerID string, size string, useCache bool) (io.ReadSeeker, error) {
	return n.FetchImage(peerID, "avatar", size, useCache)
}

// FetchHeader - fetch image header from ipfs
func (n *OpenBazaarNode) FetchHeader(peerID string, size string, useCache bool) (io.ReadSeeker, error) {
	return n.FetchImage(peerID, "header", size, useCache)
}

// FetchImage - fetch ipfs image
func (n *OpenBazaarNode) FetchImage(peerID string, imageType string, size string, useCache bool) (io.ReadSeeker, error) {
	query := "/" + peerID + "/images/" + size + "/" + imageType
	b, err := ipfs.ResolveThenCat(n.IpfsNode, ipath.FromString(query), time.Minute, n.IPNSQuorumSize, useCache)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(b), nil
}

// GetBase64Image - fetch the image and return it as base64 encoded string
func (n *OpenBazaarNode) GetBase64Image(url string) (base64ImageData, filename string, err error) {
	dial := net.Dial
	if n.TorDialer != nil {
		dial = n.TorDialer.Dial
	}
	tbTransport := &http.Transport{Dial: dial}
	client := &http.Client{Transport: tbTransport, Timeout: time.Second * 30}
	resp, err := client.Get(url)
	if err != nil {
		return "", "", err
	}
	imgBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", "", err
	}
	img := base64.StdEncoding.EncodeToString(imgBytes)
	u, err := netUrl.Parse(url)
	if err != nil {
		return "", "", err
	}
	_, filename = path.Split(u.Path)
	return img, filename, nil
}

// maybeMigrateImageHashes will iterate over the listing's images and migrate them
// to a v0 cid if they are not already v0.
func (n *OpenBazaarNode) maybeMigrateImageHashes(listing *pb.Listing) error {
	if listing.Item == nil || len(listing.Item.Images) == 0 {
		return nil
	}

	maybeMigrateImage := func(imgHash, size, filename string) (string, error) {
		id, err := cid.Decode(imgHash)
		if err != nil {
			return "", err
		}
		if id.Version() > 0 {
			newHash, err := ipfs.AddFile(n.IpfsNode, path.Join(n.RepoPath, "root", "images", size, filename))
			if err != nil {
				return "", err
			}
			return newHash, nil
		}
		return imgHash, nil
	}

	var err error
	for i, img := range listing.Item.Images {
		img.Large, err = maybeMigrateImage(img.Large, "large", img.Filename)
		if err != nil {
			return err
		}
		img.Medium, err = maybeMigrateImage(img.Medium, "medium", img.Filename)
		if err != nil {
			return err
		}
		img.Small, err = maybeMigrateImage(img.Small, "small", img.Filename)
		if err != nil {
			return err
		}
		img.Tiny, err = maybeMigrateImage(img.Tiny, "tiny", img.Filename)
		if err != nil {
			return err
		}
		img.Original, err = maybeMigrateImage(img.Original, "original", img.Filename)
		if err != nil {
			return err
		}
		listing.Item.Images[i] = img
	}
	return nil
}
