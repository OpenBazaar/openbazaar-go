package core

import (
	"encoding/json"
	"errors"
	peer "gx/ipfs/QmXYjuNuxVzXKJCfWasQk1RqkhVLDM9jtUKhqc2WPQmFSB/go-libp2p-peer"
	"time"

	"github.com/OpenBazaar/openbazaar-go/repo"
)

var (
	ipfsInventoryCacheMaxDuration = 1 * time.Hour

	ErrInventoryNotFoundForSlug = errors.New("Could not find slug in inventory")
)

// IPFSInventoryListing is the listing representation stored on IPFS
type IPFSInventoryListing struct {
	Inventory   int64  `json:"inventory"`
	LastUpdated string `json:"lastUpdated"`
}

// IPFSInventory is the complete inventory representation stored on IPFS
// It maps slug -> quantity information
type IPFSInventory map[string]*IPFSInventoryListing

// PublishInventory stores an inventory on IPFS
func (n *OpenBazaarNode) PublishInventory() error {
	listings, err := n.Datastore.Inventory().GetAll()
	if err != nil {
		return err
	}

	inventory := make(IPFSInventory, len(listings))
	var totalCount int
	for slug, variants := range listings {
		totalCount = 0
		for _, variantCount := range variants {
			totalCount += variantCount
		}

		inventory[slug] = &IPFSInventoryListing{
			Inventory:   int64(totalCount),
			LastUpdated: time.Now().UTC().Format(time.RFC3339),
		}

		go func(name string, itemInventory *IPFSInventoryListing) {
			repo.PublishObjectToIPFS(n.Context, n.IpfsNode, n.RepoPath, name, itemInventory)
		}(itemInventoryCacheName(slug), inventory[slug])
	}

	return repo.PublishObjectToIPFS(n.Context, n.IpfsNode, n.RepoPath, "inventory", inventory)
}

// GetPublishedInventoryBytes gets a byte slice representing the given peer's
// inventory that it published to IPFS
func (n *OpenBazaarNode) GetPublishedInventoryBytes(p peer.ID, useCache bool) ([]byte, error) {
	var cacheLength time.Duration
	if useCache {
		cacheLength = ipfsInventoryCacheMaxDuration
	}
	return repo.GetObjectFromIPFS(n.Context, p, "inventory", cacheLength)
}

// GetPublishedInventoryBytesForSlug gets a byte slice representing the given
// slug's inventory from IPFS
func (n *OpenBazaarNode) GetPublishedInventoryBytesForSlug(p peer.ID, slug string, useCache bool) ([]byte, error) {
	var cacheLength time.Duration
	if useCache {
		cacheLength = ipfsInventoryCacheMaxDuration
	}

	// Try loading by slug
	bytes, err := repo.GetObjectFromIPFS(n.Context, p, itemInventoryCacheName(slug), cacheLength)
	if err == nil {
		return bytes, nil
	}

	// Fall back to extracting data from the index
	bytes, err = n.GetPublishedInventoryBytes(p, useCache)
	if err != nil {
		return nil, err
	}

	inventory := IPFSInventory{}
	err = json.Unmarshal(bytes, &inventory)
	if err != nil {
		return nil, err
	}

	listingInventory, ok := inventory[slug]
	if !ok {
		return nil, ErrInventoryNotFoundForSlug
	}

	return json.Marshal(listingInventory)
}

func itemInventoryCacheName(slug string) string {
	return "inventory|" + slug
}
