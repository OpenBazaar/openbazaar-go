package core

import (
	"path"
	"os"
	"io/ioutil"
	"encoding/json"
	"crypto/sha256"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/golang/protobuf/proto"
	ec "github.com/btcsuite/btcd/btcec"
	"github.com/OpenBazaar/openbazaar-go/ipfs"
)

func (n *OpenBazaarNode) SignListing(listing *pb.Listing) (*pb.RicardianContract, error) {
	c := new(pb.RicardianContract)
	if err := validate(listing); err != nil {
		return c, err
	}
	listing.VendorID.Guid = n.IpfsNode.Identity.Pretty()
	pubkey, err := n.IpfsNode.PrivateKey.GetPublic().Bytes()
	if err != nil {
		return c, err
	}
	//TODO: add blockchain ID to listing
	listing.VendorID.Pubkeys.Guid = pubkey
	listing.VendorID.Pubkeys.Bitcoin = n.Wallet.GetMasterPublicKey().Key
	s := new(pb.Signatures)
	s.Section = pb.Signatures_LISTING
	serializedListing, err := proto.Marshal(listing)
	if err != nil {
		return c, err
	}
	guidSig, err := n.IpfsNode.PrivateKey.Sign(serializedListing)
	if err != nil {
		return c, err
	}
	priv, _ := ec.PrivKeyFromBytes(ec.S256(), n.Wallet.GetMasterPrivateKey().Key)
	hashed := sha256.Sum256(serializedListing)
	bitcoinSig, err := priv.Sign(hashed[:])
	if err != nil {
		return c, err
	}
	s.Guid = guidSig
	s.Bitcoin = bitcoinSig.Serialize()
	c.VendorListing = append(c.VendorListing, listing)
	c.Signatures = append(c.Signatures, s)
	return c, nil
}

func (n *OpenBazaarNode) UpdateListingIndex(contract *pb.RicardianContract) error {
	type listingData struct {
		Hash      string
		Name      string
	}
	indexPath:= path.Join(n.RepoPath, "node", "listings", "index.json")
	listingPath := path.Join(n.RepoPath, "node", "listings", contract.VendorListing[0].ListingName, "listing.json")
	file, _ := ioutil.ReadFile(indexPath)
	listingHash, err := ipfs.AddFile(n.Context, listingPath)
	if err != nil {
		return err
	}
	ld := listingData {
		Hash: listingHash,
		Name: contract.VendorListing[0].ListingName,
	}

	var index []listingData
	json.Unmarshal(file, &index)
	for i, d := range(index){
		if d.Name == ld.Name {
			if len(index) == 1 {
				index = []listingData{}
				break
			} else {
				index = append(index[:i], index[i + 1:]...)
			}
		}
	}


	index = append(index, ld)
	f, err := os.Create(indexPath)
	if err != nil {
		return err
	}
	defer func() {
		if err := f.Close(); err != nil {
			panic(err)
		}
	}()

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

func validate(listing *pb.Listing) error {
	// TODO: validate this listing to make sure all values are correct
	return nil
}