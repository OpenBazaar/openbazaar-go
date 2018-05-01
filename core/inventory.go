package core

import (
	"encoding/json"
	"errors"
	peer "gx/ipfs/QmXYjuNuxVzXKJCfWasQk1RqkhVLDM9jtUKhqc2WPQmFSB/go-libp2p-peer"
	"time"

	"github.com/OpenBazaar/openbazaar-go/api/notifications"
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

// GetLocalInventoryForSlug gets the local inventory for the given slug
func (n *OpenBazaarNode) GetLocalInventoryForSlug(slug string) (*IPFSInventoryListing, error) {
	variants, err := n.Datastore.Inventory().Get(slug)
	if err != nil {
		return nil, err
	}

	var inventory *IPFSInventoryListing
	var totalCount int
	for _, variantCount := range variants {
		totalCount += variantCount
	}

	inventory = &IPFSInventoryListing{
		Inventory:   int64(totalCount),
		LastUpdated: time.Now().UTC().Format(time.RFC3339),
	}

	return inventory, nil
}

// GetLocalInventory gets the inventory from the database
func (n *OpenBazaarNode) GetLocalInventory() (IPFSInventory, error) {
	listings, err := n.Datastore.Inventory().GetAll()
	if err != nil {
		return nil, err
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
	}

	return inventory, nil
}

// PublishInventory stores an inventory on IPFS
func (n *OpenBazaarNode) PublishInventory() error {
	inventory, err := n.GetLocalInventory()
	if err != nil {
		return err
	}

	n.Broadcast <- notifications.StatusNotification{"publishing"}
	go func() {
		hash, err := repo.PublishObjectToIPFS(n.Context, n.IpfsNode, n.RepoPath, "inventory", inventory)
		if err != nil {
			log.Error(err)
			n.Broadcast <- notifications.StatusNotification{"error publishing"}
			return
		}

		n.Broadcast <- notifications.StatusNotification{"publish complete"}

		err = n.sendToPushNodes(hash)
		if err != nil {
			log.Error(err)
		}
	}()

	return nil
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
	bytes, err := n.GetPublishedInventoryBytes(p, useCache)
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
