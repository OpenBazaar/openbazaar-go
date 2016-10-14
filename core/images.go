package core

import (
	"encoding/base64"
	"fmt"
	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/nfnt/resize"
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

func (n *OpenBazaarNode) SetAvatarImages(base64ImageData string) (*Images, error) {
	reader := base64.NewDecoder(base64.StdEncoding, strings.NewReader(base64ImageData))
	img, _, err := image.Decode(reader)
	if err != nil {
		return nil, err
	}
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

	ty := resize.Resize(w*50, h*50, img, resize.Lanczos3)
	sm := resize.Resize(w*100, h*100, img, resize.Lanczos3)
	md := resize.Resize(w*140, h*140, img, resize.Lanczos3)
	lg := resize.Resize(w*280, h*280, img, resize.Lanczos3)

	imgPath := path.Join(n.RepoPath, "root", "images")

	out, err := os.Create(path.Join(imgPath, "tiny", "avatar"))
	if err != nil {
		return nil, err
	}
	defer out.Close()
	jpeg.Encode(out, ty, nil)

	out, err = os.Create(path.Join(imgPath, "small", "avatar"))
	if err != nil {
		return nil, err
	}
	jpeg.Encode(out, sm, nil)

	out, err = os.Create(path.Join(imgPath, "medium", "avatar"))
	if err != nil {
		return nil, err
	}
	jpeg.Encode(out, md, nil)

	out, err = os.Create(path.Join(imgPath, "large", "avatar"))
	if err != nil {
		return nil, err
	}
	jpeg.Encode(out, lg, nil)

	out, err = os.Create(path.Join(imgPath, "original", "avatar"))
	if err != nil {
		return nil, err
	}
	jpeg.Encode(out, img, nil)

	// Add hash to profile
	t, aerr := ipfs.AddFile(n.Context, path.Join(imgPath, "tiny", "avatar"))
	if aerr != nil {
		return nil, err
	}
	s, aerr := ipfs.AddFile(n.Context, path.Join(imgPath, "small", "avatar"))
	if aerr != nil {
		return nil, err
	}
	m, aerr := ipfs.AddFile(n.Context, path.Join(imgPath, "medium", "avatar"))
	if aerr != nil {
		return nil, err
	}
	l, aerr := ipfs.AddFile(n.Context, path.Join(imgPath, "large", "avatar"))
	if aerr != nil {
		return nil, err
	}
	o, aerr := ipfs.AddFile(n.Context, path.Join(imgPath, "original", "avatar"))
	if aerr != nil {
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
	if aerr != nil {
		return nil, err
	}
	return &Images{t, s, m, l, o}, nil
}

func (n *OpenBazaarNode) SetHeaderImages(base64ImageData string) (*Images, error) {
	reader := base64.NewDecoder(base64.StdEncoding, strings.NewReader(base64ImageData))
	img, _, err := image.Decode(reader)
	if err != nil {
		return nil, err
	}
	reader = base64.NewDecoder(base64.StdEncoding, strings.NewReader(base64ImageData))
	imgCfg, _, err := image.DecodeConfig(reader)
	if err != nil {
		return nil, err
	}
	w := uint(float64(imgCfg.Width))
	h := uint(float64(imgCfg.Height))
	if w > h {
		h = h / w
		w = uint(1)
	} else if h > h {

		w = w / h
		h = uint(1)
	}

	ty := resize.Resize(w*304, h*101, img, resize.Lanczos3)
	sm := resize.Resize(w*608, h*202, img, resize.Lanczos3)
	md := resize.Resize(w*1225, h*350, img, resize.Lanczos3)
	lg := resize.Resize(w*2450, h*700, img, resize.Lanczos3)

	imgPath := path.Join(n.RepoPath, "root", "images")

	out, err := os.Create(path.Join(imgPath, "tiny", "header"))
	if err != nil {
		return nil, err
	}
	defer out.Close()
	jpeg.Encode(out, ty, nil)

	out, err = os.Create(path.Join(imgPath, "small", "header"))
	if err != nil {
		return nil, err
	}
	jpeg.Encode(out, sm, nil)

	out, err = os.Create(path.Join(imgPath, "medium", "header"))
	if err != nil {
		return nil, err
	}
	jpeg.Encode(out, md, nil)

	out, err = os.Create(path.Join(imgPath, "large", "header"))
	if err != nil {
		return nil, err
	}
	jpeg.Encode(out, lg, nil)

	out, err = os.Create(path.Join(imgPath, "original", "header"))
	if err != nil {
		return nil, err
	}
	jpeg.Encode(out, img, nil)

	// Add hash to profile
	t, aerr := ipfs.AddFile(n.Context, path.Join(imgPath, "tiny", "header"))
	if aerr != nil {
		return nil, err
	}
	s, aerr := ipfs.AddFile(n.Context, path.Join(imgPath, "small", "header"))
	if aerr != nil {
		return nil, err
	}
	m, aerr := ipfs.AddFile(n.Context, path.Join(imgPath, "medium", "header"))
	if aerr != nil {
		return nil, err
	}
	l, aerr := ipfs.AddFile(n.Context, path.Join(imgPath, "large", "header"))
	if aerr != nil {
		return nil, err
	}
	o, aerr := ipfs.AddFile(n.Context, path.Join(imgPath, "original", "header"))
	if aerr != nil {
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
	if aerr != nil {
		return nil, err
	}
	return &Images{t, s, m, l, o}, nil
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
