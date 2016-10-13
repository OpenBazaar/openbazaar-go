package core

import (
	"encoding/base64"
	"fmt"
	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/nfnt/resize"
	"github.com/oliamb/cutter"
	"image"
	"image/jpeg"
	"os"
	"path"
	"strings"
)

type Images struct {
	Tiny     string `json:"tiny"`
	Small    string `json:"small"`
	Medium   string `json:"medium"`
	Large    string `json:"large"`
	Original string `json:"original"`
}

func (n *OpenBazaarNode) SetAvatarImages(base64ImageData string) (string, error) {
	reader := base64.NewDecoder(base64.StdEncoding, strings.NewReader(base64ImageData))
	img, _, err := image.Decode(reader)
	if err != nil {
		return "", err
	}
	reader = base64.NewDecoder(base64.StdEncoding, strings.NewReader(base64ImageData))
	imgCfg, _, err := image.DecodeConfig(reader)
	if err != nil {
		return "", err
	}
	w := imgCfg.Width
	h := imgCfg.Height
	if w > h {
		w = h
	} else if h > h {
		h = w
	}
	img, err = cutter.Crop(img, cutter.Config{
		Width:  w,
		Height: h,
		Mode:   cutter.Centered,
	})
	if err != nil {
		return "", err
	}
	ty := resize.Resize(50, 50, img, resize.Lanczos3)
	sm := resize.Resize(100, 100, img, resize.Lanczos3)
	md := resize.Resize(140, 140, img, resize.Lanczos3)
	lg := resize.Resize(280, 280, img, resize.Lanczos3)
	hg := resize.Resize(560, 560, img, resize.Lanczos3)

	imgPath := path.Join(n.RepoPath, "root", "images")

	out, err := os.Create(path.Join(imgPath, "tiny", "avatar"))
	if err != nil {
		return "", err
	}
	defer out.Close()
	jpeg.Encode(out, ty, nil)

	out, err = os.Create(path.Join(imgPath, "small", "avatar"))
	if err != nil {
		return "", err
	}
	jpeg.Encode(out, sm, nil)

	out, err = os.Create(path.Join(imgPath, "medium", "avatar"))
	if err != nil {
		return "", err
	}
	jpeg.Encode(out, md, nil)

	out, err = os.Create(path.Join(imgPath, "large", "avatar"))
	if err != nil {
		return "", err
	}
	jpeg.Encode(out, lg, nil)

	out, err = os.Create(path.Join(imgPath, "huge", "avatar"))
	if err != nil {
		return "", err
	}
	jpeg.Encode(out, hg, nil)

	// Add hash to profile
	hash, aerr := ipfs.AddFile(n.Context, path.Join(imgPath, "huge", "avatar"))
	if aerr != nil {
		return "", err
	}
	profile, err := n.GetProfile()
	if err != nil {
		return "", err
	}
	profile.AvatarHash = hash
	err = n.UpdateProfile(&profile)
	if aerr != nil {
		return "", err
	}
	return hash, nil
}

func (n *OpenBazaarNode) SetHeaderImages(base64ImageData string) (string, error) {
	reader := base64.NewDecoder(base64.StdEncoding, strings.NewReader(base64ImageData))
	img, _, err := image.Decode(reader)
	if err != nil {
		return "", err
	}
	reader = base64.NewDecoder(base64.StdEncoding, strings.NewReader(base64ImageData))
	imgCfg, _, err := image.DecodeConfig(reader)
	if err != nil {
		return "", err
	}
	w := float64(imgCfg.Width)
	h := float64(imgCfg.Height)
	if w/h > 3.5 {
		w = h * 3.5
	} else if w/h < 3.5 {
		h = w / 3.5
	}
	img, err = cutter.Crop(img, cutter.Config{
		Width:  int(w),
		Height: int(h),
		Mode:   cutter.Centered,
	})
	if err != nil {
		return "", err
	}
	ty := resize.Resize(304, 101, img, resize.Lanczos3)
	sm := resize.Resize(608, 202, img, resize.Lanczos3)
	md := resize.Resize(1225, 350, img, resize.Lanczos3)
	lg := resize.Resize(2450, 700, img, resize.Lanczos3)
	hg := resize.Resize(4900, 1400, img, resize.Lanczos3)

	imgPath := path.Join(n.RepoPath, "root", "images")

	out, err := os.Create(path.Join(imgPath, "tiny", "header"))
	if err != nil {
		return "", err
	}
	defer out.Close()
	jpeg.Encode(out, ty, nil)

	out, err = os.Create(path.Join(imgPath, "small", "header"))
	if err != nil {
		return "", err
	}
	jpeg.Encode(out, sm, nil)

	out, err = os.Create(path.Join(imgPath, "medium", "header"))
	if err != nil {
		return "", err
	}
	jpeg.Encode(out, md, nil)

	out, err = os.Create(path.Join(imgPath, "large", "header"))
	if err != nil {
		return "", err
	}
	jpeg.Encode(out, lg, nil)

	out, err = os.Create(path.Join(imgPath, "huge", "header"))
	if err != nil {
		return "", err
	}
	jpeg.Encode(out, hg, nil)

	// Add hash to profile
	hash, aerr := ipfs.AddFile(n.Context, path.Join(imgPath, "large", "header"))
	if aerr != nil {
		return "", err
	}
	profile, err := n.GetProfile()
	if err != nil {
		return "", err
	}
	profile.HeaderHash = hash
	err = n.UpdateProfile(&profile)
	if aerr != nil {
		return "", err
	}
	return hash, nil
}

func (n *OpenBazaarNode) SetProductImages(base64ImageData, filename string) (*Images, error) {
	// Decode base64 image data
	reader := base64.NewDecoder(base64.StdEncoding, strings.NewReader(base64ImageData))
	img, _, err := image.Decode(reader)
	if err != nil {
		fmt.Println(1)
		return nil, err
	}

	// Get the image config
	reader = base64.NewDecoder(base64.StdEncoding, strings.NewReader(base64ImageData))
	imgCfg, _, err := image.DecodeConfig(reader)
	if err != nil {
		return nil, err
	}
	w := uint(imgCfg.Width)
	h := uint(imgCfg.Height)
	if w > h {
		w = w / h
		h = uint(1)
	} else if h > h {
		h = h / w
		w = uint(1)
	}

	ty := resize.Resize(w*60, h*60, img, resize.Lanczos3)
	sm := resize.Resize(w*228, h*228, img, resize.Lanczos3)
	md := resize.Resize(w*500, h*500, img, resize.Lanczos3)
	lg := resize.Resize(w*1000, h*1000, img, resize.Lanczos3)

	imgPath := path.Join(n.RepoPath, "root", "images")

	out, err := os.Create(path.Join(imgPath, "tiny", filename))
	if err != nil {
		return nil, err
	}
	defer out.Close()
	jpeg.Encode(out, ty, nil)

	out, err = os.Create(path.Join(imgPath, "small", filename))
	if err != nil {
		return nil, err
	}
	jpeg.Encode(out, sm, nil)

	out, err = os.Create(path.Join(imgPath, "medium", filename))
	if err != nil {
		return nil, err
	}
	jpeg.Encode(out, md, nil)

	out, err = os.Create(path.Join(imgPath, "large", filename))
	if err != nil {
		return nil, err
	}
	jpeg.Encode(out, lg, nil)

	out, err = os.Create(path.Join(imgPath, "original", filename))
	if err != nil {
		return nil, err
	}
	jpeg.Encode(out, img, nil)

	// Get the image hashes
	t, aerr := ipfs.AddFile(n.Context, path.Join(imgPath, "tiny", filename))
	if aerr != nil {
		return nil, err
	}
	s, aerr := ipfs.AddFile(n.Context, path.Join(imgPath, "small", filename))
	if aerr != nil {
		return nil, err
	}
	m, aerr := ipfs.AddFile(n.Context, path.Join(imgPath, "medium", filename))
	if aerr != nil {
		return nil, err
	}
	l, aerr := ipfs.AddFile(n.Context, path.Join(imgPath, "large", filename))
	if aerr != nil {
		return nil, err
	}
	o, aerr := ipfs.AddFile(n.Context, path.Join(imgPath, "original", filename))
	if aerr != nil {
		return nil, err
	}
	ret := &Images{t, s, m, l, o}
	return ret, nil
}
