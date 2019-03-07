package core

import (
	"encoding/base64"
	"gx/ipfs/QmPSQnBKM9g7BaUcZCvswUJVscQ1ipjmwxN5PXCjkp9EQ7/go-cid"
	"image" // load gif
	_ "image/gif"
	"image/jpeg" // load png
	_ "image/png"
	"io/ioutil"
	"net"
	"net/http"
	netUrl "net/url"
	"os"
	"path"
	"strings"
	"time"

	ipath "gx/ipfs/QmT3rzed1ppXefourpmoZ7tyVQfsGPQZ1pHDngLmCvXxd3/go-path"
	"gx/ipfs/QmfB3oNXGGq9S4B2a9YeCajoATms3Zw2VvDm8fK7VeLSV8/go-unixfs/io"

	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/OpenBazaar/openbazaar-go/pb"
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

// SetProductImages - use the original image ina base64 string format and generate tiny,
// small, medium and large images for the product
func (n *OpenBazaarNode) SetProductImages(base64ImageData, filename string) (*pb.Profile_Image, error) {
	return n.resizeImage(base64ImageData, filename, 120, 120)
}

func (n *OpenBazaarNode) resizeImage(base64ImageData, filename string, baseWidth, baseHeight uint) (*pb.Profile_Image, error) {
	img, imgCfg, err := decodeImageData(base64ImageData)
	if err != nil {
		return nil, err
	}

	imgPath := path.Join(n.RepoPath, "root", "images")

	t, err := n.addResizedImage(img, imgCfg, 1*baseWidth, 1*baseHeight, path.Join(imgPath, "tiny", filename))
	if err != nil {
		return nil, err
	}
	s, err := n.addResizedImage(img, imgCfg, 2*baseWidth, 2*baseHeight, path.Join(imgPath, "small", filename))
	if err != nil {
		return nil, err
	}
	m, err := n.addResizedImage(img, imgCfg, 4*baseWidth, 4*baseHeight, path.Join(imgPath, "medium", filename))
	if err != nil {
		return nil, err
	}
	l, err := n.addResizedImage(img, imgCfg, 8*baseWidth, 8*baseHeight, path.Join(imgPath, "large", filename))
	if err != nil {
		return nil, err
	}
	o, err := n.addImage(img, path.Join(imgPath, "original", filename))
	if err != nil {
		return nil, err
	}

	return &pb.Profile_Image{Tiny: t, Small: s, Medium: m, Large: l, Original: o}, nil
}

func (n *OpenBazaarNode) addImage(img image.Image, imgPath string) (string, error) {
	out, err := os.Create(imgPath)
	if err != nil {
		return "", err
	}
	jpeg.Encode(out, img, nil)
	out.Close()
	return ipfs.AddFile(n.IpfsNode, imgPath)
}

func (n *OpenBazaarNode) addResizedImage(img image.Image, imgCfg *image.Config, w, h uint, imgPath string) (string, error) {
	width, height := getImageAttributes(w, h, uint(imgCfg.Width), uint(imgCfg.Height))
	newImg := resize.Resize(width, height, img, resize.Lanczos3)
	return n.addImage(newImg, imgPath)
}

func decodeImageData(base64ImageData string) (image.Image, *image.Config, error) {
	reader := base64.NewDecoder(base64.StdEncoding, strings.NewReader(base64ImageData))
	img, _, err := image.Decode(reader)
	if err != nil {
		return nil, nil, err
	}
	reader = base64.NewDecoder(base64.StdEncoding, strings.NewReader(base64ImageData))
	imgCfg, _, err := image.DecodeConfig(reader)
	if err != nil {
		return nil, nil, err
	}
	return img, &imgCfg, err
}

func getImageAttributes(targetWidth, targetHeight, imgWidth, imgHeight uint) (width, height uint) {
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
	return uint(w), uint(h)
}

// FetchAvatar - fetch image avatar from ipfs
func (n *OpenBazaarNode) FetchAvatar(peerID string, size string, useCache bool) (io.DagReader, error) {
	return n.FetchImage(peerID, "avatar", size, useCache)
}

// FetchHeader - fetch image header from ipfs
func (n *OpenBazaarNode) FetchHeader(peerID string, size string, useCache bool) (io.DagReader, error) {
	return n.FetchImage(peerID, "header", size, useCache)
}

// FetchImage - fetch ipfs image
func (n *OpenBazaarNode) FetchImage(peerID string, imageType string, size string, useCache bool) (io.DagReader, error) {
	query := "/" + peerID + "/images/" + size + "/" + imageType
	b, err := n.IPNSResolveThenCat(ipath.FromString(query), time.Minute, useCache)
	if err != nil {
		return nil, err
	}
	return io.NewBufDagReader(b), nil
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

	for i, image := range listing.Item.Images {
		largeID, err := cid.Decode(image.Large)
		if err != nil {
			return err
		}
		if largeID.Version() > 0 {
			hash, err := ipfs.AddFile(n.IpfsNode, path.Join(n.RepoPath, "root", "images", "large", image.Filename))
			if err != nil {
				return err
			}
			image.Large = hash
		}
		mediumID, err := cid.Decode(image.Medium)
		if err != nil {
			return err
		}
		if mediumID.Version() > 0 {
			hash, err := ipfs.AddFile(n.IpfsNode, path.Join(n.RepoPath, "root", "images", "medium", image.Filename))
			if err != nil {
				return err
			}
			image.Medium = hash
		}
		smallID, err := cid.Decode(image.Small)
		if err != nil {
			return err
		}
		if smallID.Version() > 0 {
			hash, err := ipfs.AddFile(n.IpfsNode, path.Join(n.RepoPath, "root", "images", "small", image.Filename))
			if err != nil {
				return err
			}
			image.Small = hash
		}
		tinyID, err := cid.Decode(image.Tiny)
		if err != nil {
			return err
		}
		if tinyID.Version() > 0 {
			hash, err := ipfs.AddFile(n.IpfsNode, path.Join(n.RepoPath, "root", "images", "tiny", image.Filename))
			if err != nil {
				return err
			}
			image.Tiny = hash
		}
		originalID, err := cid.Decode(image.Original)
		if err != nil {
			return err
		}
		if originalID.Version() > 0 {
			hash, err := ipfs.AddFile(n.IpfsNode, path.Join(n.RepoPath, "root", "images", "original", image.Filename))
			if err != nil {
				return err
			}
			image.Original = hash
		}
		listing.Item.Images[i] = image
	}
	return nil
}
