package core

import (
	"encoding/json"
	"errors"
	"math/big"

	peer "gx/ipfs/QmYVXrKrKHDC9FobgmcmshCDyWwdrfwfanNQN4oxJ9Fk3h/go-libp2p-peer"
	"time"

	"github.com/OpenBazaar/openbazaar-go/repo"
)

var (
	ipfsInventoryCacheMaxDuration = 1 * time.Hour
	// ErrInventoryNotFoundForSlug - inventory not found error
	ErrInventoryNotFoundForSlug = errors.New("could not find slug in inventory")
)

// InventoryListing is the listing representation stored on IPFS
type InventoryListing struct {
	Inventory   string `json:"inventory"`
	LastUpdated string `json:"lastUpdated"`
}

// Inventory is the complete inventory representation stored on IPFS
// It maps slug -> quantity information
type Inventory map[string]*InventoryListing

// GetLocalInventory gets the inventory from the database
func (n *OpenBazaarNode) GetLocalInventory() (Inventory, error) {
	listings, err := n.Datastore.Inventory().GetAll()
	if err != nil {
		return nil, err
	}

	var (
		totalCount *big.Int
		inventory  = make(Inventory, len(listings))
	)
	for slug, variants := range listings {
		totalCount = big.NewInt(0)
		for _, variantCount := range variants {
			totalCount = new(big.Int).Add(totalCount, variantCount)
		}

		inventory[slug] = &InventoryListing{
			Inventory:   totalCount.String(),
			LastUpdated: time.Now().UTC().Format(time.RFC3339),
		}
	}

	return inventory, nil
}

// GetLocalInventoryForSlug gets the local inventory for the given slug
func (n *OpenBazaarNode) GetLocalInventoryForSlug(slug string) (*InventoryListing, error) {
	variants, err := n.Datastore.Inventory().Get(slug)
	if err != nil {
		return nil, err
	}

	var inventory *InventoryListing
	totalCount := big.NewInt(0)
	for _, variantCount := range variants {
		totalCount = new(big.Int).Add(totalCount, variantCount)
	}

	inventory = &InventoryListing{
		Inventory:   totalCount.String(),
		LastUpdated: time.Now().UTC().Format(time.RFC3339),
	}

	return inventory, nil
}

// PublishInventory stores an inventory on IPFS
func (n *OpenBazaarNode) PublishInventory() error {
	// TODO: [cp] need to refactor the inventory publishing and getting as we've discussed before
	// now is a good time to do that.
	/*inventory, err := n.GetLocalInventory()
	if err != nil {
		return err
	}
	n.Broadcast <- repo.StatusNotification{Status: "publishing"}
	go func() {
		hash, err := repo.PublishObjectToIPFS(n.IpfsNode, n.RepoPath, "inventory", inventory)
		if err != nil {
			log.Error(err)
			n.Broadcast <- repo.StatusNotification{Status: "error publishing"}
			return
		}

		n.Broadcast <- repo.StatusNotification{Status: "publish complete"}

		err = n.sendToPushNodes(hash)
		if err != nil {
			log.Error(err)
		}
	}()*/

	return nil
}

// GetPublishedInventoryBytes gets a byte slice representing the given peer's
// inventory that it published to IPFS
func (n *OpenBazaarNode) GetPublishedInventoryBytes(p peer.ID, useCache bool) ([]byte, error) {
	var cacheLength time.Duration
	if useCache {
		cacheLength = ipfsInventoryCacheMaxDuration
	}
	return repo.GetObjectFromIPFS(n.IpfsNode, p, "inventory", cacheLength)
}

// GetPublishedInventoryBytesForSlug gets a byte slice representing the given
// slug's inventory from IPFS
func (n *OpenBazaarNode) GetPublishedInventoryBytesForSlug(p peer.ID, slug string, useCache bool) ([]byte, error) {
	bytes, err := n.GetPublishedInventoryBytes(p, useCache)
	if err != nil {
		return nil, err
	}

	inventory := Inventory{}
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
