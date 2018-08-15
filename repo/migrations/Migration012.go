package migrations

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"path"

	"gx/ipfs/QmaPbCnUMBohSGo3KnxEa2bHqyJVVeEEcwtqJAYxerieBo/go-libp2p-crypto"

	"github.com/OpenBazaar/jsonpb"
	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/golang/protobuf/proto"
	ipfscore "github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/repo/fsrepo"
)

const migration012_ListingVersionForMarkupListings = 4

type Migration012 struct{}

type Migration012_ListingData struct {
	Hash         string   `json:"hash"`
	Slug         string   `json:"slug"`
	Title        string   `json:"title"`
	Categories   []string `json:"categories"`
	NSFW         bool     `json:"nsfw"`
	ContractType string   `json:"contractType"`
	Description  string   `json:"description"`
	Thumbnail    struct {
		Tiny   string `json:"tiny"`
		Small  string `json:"small"`
		Medium string `json:"medium"`
	} `json:"thumbnail"`
	Price struct {
		CurrencyCode string  `json:"currencyCode"`
		Amount       uint64  `json:"amount"`
		Modifier     float32 `json:"modifier"`
	} `json:"price"`
	ShipsTo            []string `json:"shipsTo"`
	FreeShipping       []string `json:"freeShipping"`
	Language           string   `json:"language"`
	AverageRating      float32  `json:"averageRating"`
	RatingCount        uint32   `json:"ratingCount"`
	ModeratorIDs       []string `json:"moderators"`
	AcceptedCurrencies []string `json:"acceptedCurrencies"`
	CoinType           string   `json:"coinType"`
}

func Migration012_listingHasNewFeaturesAndOldVersion(sl *pb.SignedListing) bool {
	metadata := sl.Listing.Metadata
	if metadata == nil {
		return false
	}
	return metadata.Version == 3 &&
		metadata.PriceModifier != 0
}

func Migration012_GetIdentityKey(repoPath, databasePassword string, testnetEnabled bool) ([]byte, error) {
	db, err := OpenDB(repoPath, databasePassword, testnetEnabled)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	var identityKey []byte
	err = db.
		QueryRow("select value from config where key=?", "identityKey").
		Scan(&identityKey)
	if err != nil {
		return nil, err
	}
	return identityKey, nil
}

func (Migration012) Up(repoPath, databasePassword string, testnetEnabled bool) error {
	listingsIndexFilePath := path.Join(repoPath, "root", "listings.json")

	// Find all crypto listings
	if _, err := os.Stat(listingsIndexFilePath); os.IsNotExist(err) {
		// Finish early if no listings are found
		return writeRepoVer(repoPath, 13)
	}
	listingsIndexJSONBytes, err := ioutil.ReadFile(listingsIndexFilePath)
	if err != nil {
		return err
	}

	listingsIndex := []*Migration012_ListingData{}
	err = json.Unmarshal(listingsIndexJSONBytes, &listingsIndex)
	if err != nil {
		return err
	}

	cryptoListings := []*Migration012_ListingData{}
	for _, listingAbstract := range listingsIndex {
		if listingAbstract.ContractType == "CRYPTOCURRENCY" {
			cryptoListings = append(cryptoListings, listingAbstract)
		}
	}

	// Finish early If no crypto listings
	if len(cryptoListings) == 0 {
		return writeRepoVer(repoPath, 13)
	}

	// Check each crypto listing for markup
	markupListings := []*pb.SignedListing{}
	for _, listingAbstract := range cryptoListings {
		listingJSONBytes, err := ioutil.ReadFile(migration012_listingFilePath(repoPath, listingAbstract.Slug))
		if err != nil {
			return err
		}
		sl := new(pb.SignedListing)
		err = jsonpb.UnmarshalString(string(listingJSONBytes), sl)
		if err != nil {
			return err
		}

		if Migration012_listingHasNewFeaturesAndOldVersion(sl) {
			markupListings = append(markupListings, sl)
		}
	}

	// Finish early If no crypto listings with new features are found
	if len(markupListings) == 0 {
		return writeRepoVer(repoPath, 13)
	}

	// Setup signing capabilities
	identityKey, err := Migration012_GetIdentityKey(repoPath, databasePassword, testnetEnabled)
	if err != nil {
		return err
	}

	identity, err := ipfs.IdentityFromKey(identityKey)
	if err != nil {
		return err
	}

	// IPFS node setup
	r, err := fsrepo.Open(repoPath)
	if err != nil {
		return err
	}
	cctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg, err := r.Config()
	if err != nil {
		return err
	}

	cfg.Identity = identity

	ncfg := &ipfscore.BuildCfg{
		Repo:   r,
		Online: false,
		ExtraOpts: map[string]bool{
			"mplex": true,
		},
		DNSResolver: nil,
		Routing:     nil,
	}

	nd, err := ipfscore.NewNode(cctx, ncfg)
	if err != nil {
		return err
	}
	defer nd.Close()

	// Update each listing to have the latest version number and resave
	// Save the new hashes for each changed listing so we can update the index.
	hashes := make(map[string]string)

	privKey, err := crypto.UnmarshalPrivateKey(identityKey)
	if err != nil {
		return err
	}

	for _, sl := range markupListings {
		sl.Listing.Metadata.Version = migration012_ListingVersionForMarkupListings

		serializedListing, err := proto.Marshal(sl.Listing)
		if err != nil {
			return err
		}

		idSig, err := privKey.Sign(serializedListing)
		if err != nil {
			return err
		}
		sl.Signature = idSig

		m := jsonpb.Marshaler{
			EnumsAsInts:  false,
			EmitDefaults: false,
			Indent:       "    ",
			OrigName:     false,
		}
		out, err := m.MarshalToString(sl)
		if err != nil {
			return err
		}

		filename := migration012_listingFilePath(repoPath, sl.Listing.Slug)
		if err := ioutil.WriteFile(filename, []byte(out), os.ModePerm); err != nil {
			return err
		}
		h, err := ipfs.GetHashOfFile(nd, filename)
		if err != nil {
			return err
		}
		hashes[sl.Listing.Slug] = h
	}

	// Update listing index
	indexBytes, err := ioutil.ReadFile(listingsIndexFilePath)
	if err != nil {
		return err
	}
	var index []Migration012_ListingData

	err = json.Unmarshal(indexBytes, &index)
	if err != nil {
		return err
	}

	for i, l := range index {
		h, ok := hashes[l.Slug]

		// Not one of the changed listings
		if !ok {
			continue
		}

		l.Hash = h
		index[i] = l
	}

	// Write it back to file
	ifile, err := os.Create(listingsIndexFilePath)
	if err != nil {
		return err
	}
	defer ifile.Close()

	j, jerr := json.MarshalIndent(index, "", "    ")
	if jerr != nil {
		return jerr
	}
	_, werr := ifile.Write(j)
	if werr != nil {
		return werr
	}

	_, err = ipfs.GetHashOfFile(nd, listingsIndexFilePath)
	if err != nil {
		return err
	}

	return writeRepoVer(repoPath, 13)
}

func migration012_listingFilePath(datadir string, slug string) string {
	return path.Join(datadir, "root", "listings", slug+".json")
}

func (Migration012) Down(repoPath, databasePassword string, testnetEnabled bool) error {
	return writeRepoVer(repoPath, 12)
}
