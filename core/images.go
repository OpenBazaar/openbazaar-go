package core

import (
	"encoding/base64"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"net"
	"os"
	"path"
	"strings"

	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/ipfs/go-ipfs/core/coreunix"
	ipnspb "github.com/ipfs/go-ipfs/namesys/pb"
	ipnspath "github.com/ipfs/go-ipfs/path"
	"github.com/ipfs/go-ipfs/unixfs/io"
	"github.com/nfnt/resize"
	"golang.org/x/net/context"
	u "gx/ipfs/QmSU6eubNdhXjFBJBSksTp8kv8YRub8mGAPv8tVJHmL2EU/go-ipfs-util"
	ds "gx/ipfs/QmVSase1JP7cq9QkPT46oNwdp9pT6kBkG3oqS14y3QcZjG/go-datastore"
	proto "gx/ipfs/QmZ4Qi3GaRbjcx28Sme5eMH7RQjGkt8wHxt2a65oLaeFEV/gogo-protobuf/proto"
	"io/ioutil"
	"net/http"
	netUrl "net/url"
	"time"
)

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

	return &pb.Profile_Image{t, s, m, l, o}, nil
}

func (n *OpenBazaarNode) addImage(img image.Image, imgPath string) (string, error) {
	out, err := os.Create(imgPath)
	if err != nil {
		return "", err
	}
	jpeg.Encode(out, img, nil)
	out.Close()
	return ipfs.AddFile(n.Context, imgPath)
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

func (n *OpenBazaarNode) FetchAvatar(peerId string, size string, useCache bool) (io.DagReader, error) {
	return n.FetchImage(peerId, "avatar", size, useCache)
}

func (n *OpenBazaarNode) FetchHeader(peerId string, size string, useCache bool) (io.DagReader, error) {
	return n.FetchImage(peerId, "header", size, useCache)
}

func (n *OpenBazaarNode) FetchImage(peerId string, imageType string, size string, useCache bool) (io.DagReader, error) {
	fetch := func(rootHash string) (io.DagReader, error) {
		var dr io.DagReader
		var err error

		ctx, cancel := context.WithTimeout(context.Background(), time.Minute*2)
		defer cancel()
		if rootHash == "" {
			query := "/ipns/" + peerId + "/images/" + size + "/" + imageType
			dr, err = coreunix.Cat(ctx, n.IpfsNode, query)
			if err != nil {
				return dr, err
			}
		} else {
			query := "/ipfs/" + rootHash + "/images/" + size + "/" + imageType
			dr, err = coreunix.Cat(ctx, n.IpfsNode, query)
			if err != nil {
				return dr, err
			}
		}
		return dr, nil
	}

	var dr io.DagReader
	var err error
	var recordAvailable bool
	var val interface{}
	if useCache {
		val, err = n.IpfsNode.Repo.Datastore().Get(ds.NewKey(cachePrefix + peerId))
		if err != nil { // No record in datastore
			dr, err = fetch("")
			if err != nil {
				return dr, err
			}
		} else { // Record available, let's see how old it is
			entry := new(ipnspb.IpnsEntry)
			err = proto.Unmarshal(val.([]byte), entry)
			if err != nil {
				return dr, err
			}
			p, err := ipnspath.ParsePath(string(entry.GetValue()))
			if err != nil {
				return dr, err
			}
			eol, ok := checkEOL(entry)
			if ok && eol.Before(time.Now()) { // Too old, fetch new profile
				dr, err = fetch("")
			} else { // Relatively new, we can do a standard IPFS query (which should be cached)
				dr, err = fetch(strings.TrimPrefix(p.String(), "/ipfs/"))
				// Let's now try to get the latest record in a new goroutine so it's available next time
				go fetch("")
			}
			if err != nil {
				return dr, err
			}
			recordAvailable = true
		}
	} else {
		dr, err = fetch("")
		if err != nil {
			return dr, err
		}
		recordAvailable = false
	}

	// Update the record with a new EOL
	go func() {
		if !recordAvailable {
			val, err = n.IpfsNode.Repo.Datastore().Get(ds.NewKey(cachePrefix + peerId))
			if err != nil {
				return
			}
		}
		entry := new(ipnspb.IpnsEntry)
		err = proto.Unmarshal(val.([]byte), entry)
		if err != nil {
			return
		}
		entry.Validity = []byte(u.FormatRFC3339(time.Now().Add(CachedProfileTime)))
		v, err := proto.Marshal(entry)
		if err != nil {
			return
		}
		n.IpfsNode.Repo.Datastore().Put(ds.NewKey(cachePrefix+peerId), v)
	}()
	return dr, nil
}

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
