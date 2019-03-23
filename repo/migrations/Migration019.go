package migrations

import (
	"encoding/json"
	"fmt"
	"gx/ipfs/QmPSQnBKM9g7BaUcZCvswUJVscQ1ipjmwxN5PXCjkp9EQ7/go-cid"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/core/coreunix"

	"gx/ipfs/QmZMWMvWMVKCbHetJ4RgndbuEF1io2UpUxwQwtNjtYPzSC/go-ipfs-files"

	"github.com/ipfs/go-ipfs/core/mock"
)

// Circular imports importing the core package here so we need to copy this here.
type price struct {
	CurrencyCode string  `json:"currencyCode"`
	Amount       uint64  `json:"amount"`
	Modifier     float32 `json:"modifier"`
}
type thumbnail struct {
	Tiny   string `json:"tiny"`
	Small  string `json:"small"`
	Medium string `json:"medium"`
}

type ListingData struct {
	Hash               string    `json:"hash"`
	Slug               string    `json:"slug"`
	Title              string    `json:"title"`
	Categories         []string  `json:"categories"`
	NSFW               bool      `json:"nsfw"`
	ContractType       string    `json:"contractType"`
	Description        string    `json:"description"`
	Thumbnail          thumbnail `json:"thumbnail"`
	Price              price     `json:"price"`
	ShipsTo            []string  `json:"shipsTo"`
	FreeShipping       []string  `json:"freeShipping"`
	Language           string    `json:"language"`
	AverageRating      float32   `json:"averageRating"`
	RatingCount        uint32    `json:"ratingCount"`
	ModeratorIDs       []string  `json:"moderators"`
	AcceptedCurrencies []string  `json:"acceptedCurrencies"`
	CoinType           string    `json:"coinType"`
}

// Migration019 migrates the listing index file to use the new style (Qm) hashes
// for listings rather than the old CID (z) style hashes.
type Migration019 struct{}

func (Migration019) Up(repoPath string, dbPassword string, testnet bool) error {
	ipfsNode, err := coremock.NewMockNode()
	if err != nil {
		return err
	}

	err = updateEachListingOnIndex(repoPath, func(ld *ListingData) error {
		listingPath := path.Join(repoPath, "root", "listings", ld.Slug+".json")

		listingHash, err := getHashOfFile(ipfsNode, listingPath)
		if err != nil {
			return err
		}
		ld.Hash = listingHash
		return nil
	})
	if err != nil {
		return err
	}

	if err := writeRepoVer(repoPath, 20); err != nil {
		return fmt.Errorf("bumping repover to 19: %s", err.Error())
	}
	return nil
}

func (Migration019) Down(repoPath string, dbPassword string, testnet bool) error {
	// Down migration is a no-op (outside of updating the version).
	// We can't calculate the old hashes because the go-ipfs is not configured to
	// do so.
	return writeRepoVer(repoPath, 19)
}

func updateEachListingOnIndex(repoPath string, updateListing func(*ListingData) error) error {
	indexPath := path.Join(repoPath, "root", "listings.json")

	var index []ListingData

	_, ferr := os.Stat(indexPath)
	if os.IsNotExist(ferr) {
		return nil
	}
	file, err := ioutil.ReadFile(indexPath)
	if err != nil {
		return err
	}
	err = json.Unmarshal(file, &index)
	if err != nil {
		return err
	}

	for i, d := range index {
		if err := updateListing(&d); err != nil {
			return err
		}
		index[i] = d
	}

	f, err := os.Create(indexPath)
	if err != nil {
		return err
	}
	defer f.Close()

	j, jerr := json.MarshalIndent(index, "", "    ")
	if jerr != nil {
		return jerr
	}
	_, werr := f.Write(j)
	if werr != nil {
		return werr
	}
	return nil
}

func getHashOfFile(n *core.IpfsNode, root string) (rootHash string, err error) {
	defer n.Blockstore.PinLock().Unlock()

	stat, err := os.Lstat(root)
	if err != nil {
		return "", err
	}

	f, err := files.NewSerialFile(filepath.Base(root), root, false, stat)
	if err != nil {
		return "", err
	}
	defer f.Close()

	fileAdder, err := coreunix.NewAdder(n.Context(), n.Pinning, n.Blockstore, n.DAG)
	if err != nil {
		return "", err
	}

	fileAdder.Progress = false
	fileAdder.Hidden = true
	fileAdder.Pin = true
	fileAdder.Trickle = false
	fileAdder.Wrap = false
	fileAdder.Chunker = ""
	fileAdder.CidBuilder = cid.V0Builder{}

	node, err := fileAdder.AddAllAndPin(f)
	if err != nil {
		return "", err
	}
	return node.Cid().String(), nil
}
