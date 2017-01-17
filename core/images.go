package core

import (
	"encoding/base64"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"os"
	"path"
	"strings"

	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/nfnt/resize"
)

type Images struct {
	Tiny     string `json:"tiny"`
	Small    string `json:"small"`
	Medium   string `json:"medium"`
	Large    string `json:"large"`
	Original string `json:"original"`
}

func (n *OpenBazaarNode) SetAvatarImages(base64ImageData string) (*Images, error) {
	img, imgCfg, err := decodeImageData(base64ImageData)
	if err != nil {
		return nil, err
	}

	imgPath := path.Join(n.RepoPath, "root", "images")

	t, err := n.addResizedImage(img, imgCfg, 50, 50, path.Join(imgPath, "tiny", "avatar"))
	if err != nil {
		return nil, err
	}
	s, err := n.addResizedImage(img, imgCfg, 100, 100, path.Join(imgPath, "small", "avatar"))
	if err != nil {
		return nil, err
	}
	m, err := n.addResizedImage(img, imgCfg, 140, 140, path.Join(imgPath, "medium", "avatar"))
	if err != nil {
		return nil, err
	}
	l, err := n.addResizedImage(img, imgCfg, 280, 280, path.Join(imgPath, "large", "avatar"))
	if err != nil {
		return nil, err
	}
	o, err := n.addImage(img, path.Join(imgPath, "original", "avatar"))
	if err != nil {
		return nil, err
	}

	profile, err := n.GetProfile()
	if err != nil {
		return nil, err
	}
	i := new(pb.Profile_Image)
	i.Tiny = t
	i.Small = s
	i.Medium = m
	i.Large = l
	i.Original = o
	profile.AvatarHashes = i
	err = n.UpdateProfile(&profile)
	if err != nil {
		return nil, err
	}
	return &Images{t, s, m, l, o}, nil
}

func (n *OpenBazaarNode) SetHeaderImages(base64ImageData string) (*Images, error) {
	img, imgCfg, err := decodeImageData(base64ImageData)
	if err != nil {
		return nil, err
	}

	imgPath := path.Join(n.RepoPath, "root", "images")

	t, err := n.addResizedImage(img, imgCfg, 304, 87, path.Join(imgPath, "tiny", "header"))
	if err != nil {
		return nil, err
	}
	s, err := n.addResizedImage(img, imgCfg, 608, 174, path.Join(imgPath, "small", "header"))
	if err != nil {
		return nil, err
	}
	m, err := n.addResizedImage(img, imgCfg, 1225, 350, path.Join(imgPath, "medium", "header"))
	if err != nil {
		return nil, err
	}
	l, err := n.addResizedImage(img, imgCfg, 2450, 700, path.Join(imgPath, "large", "header"))
	if err != nil {
		return nil, err
	}
	o, err := n.addImage(img, path.Join(imgPath, "original", "header"))
	if err != nil {
		return nil, err
	}
	profile, err := n.GetProfile()
	if err != nil {
		return nil, err
	}
	i := new(pb.Profile_Image)
	i.Tiny = t
	i.Small = s
	i.Medium = m
	i.Large = l
	i.Original = o
	profile.HeaderHashes = i
	err = n.UpdateProfile(&profile)
	if err != nil {
		return nil, err
	}
	return &Images{t, s, m, l, o}, nil
}

func (n *OpenBazaarNode) SetProductImages(base64ImageData, filename string) (*Images, error) {
	img, imgCfg, err := decodeImageData(base64ImageData)
	if err != nil {
		return nil, err
	}

	imgPath := path.Join(n.RepoPath, "root", "images")

	t, err := n.addResizedImage(img, imgCfg, 60, 60, path.Join(imgPath, "tiny", filename))
	if err != nil {
		return nil, err
	}
	s, err := n.addResizedImage(img, imgCfg, 228, 228, path.Join(imgPath, "small", filename))
	if err != nil {
		return nil, err
	}
	m, err := n.addResizedImage(img, imgCfg, 500, 500, path.Join(imgPath, "medium", filename))
	if err != nil {
		return nil, err
	}
	l, err := n.addResizedImage(img, imgCfg, 1000, 1000, path.Join(imgPath, "large", filename))
	if err != nil {
		return nil, err
	}
	o, err := n.addImage(img, path.Join(imgPath, "original", filename))
	if err != nil {
		return nil, err
	}

	return &Images{t, s, m, l, o}, nil
}

func (n *OpenBazaarNode) addImage(img image.Image, imgPath string) (string, error) {
	out, err := os.Create(imgPath)
	if err != nil {
		return "", err
	}
	defer out.Close()
	jpeg.Encode(out, img, nil)
	return ipfs.GetHash(n.Context, imgPath)
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
