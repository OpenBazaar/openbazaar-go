package core

import (
	"encoding/json"
	"errors"
	"fmt"
	cid "gx/ipfs/QmNp85zy9RLrQ5oQD4hPyS39ezrrXpcaa7R4Y9kxdWQLLQ/go-cid"
	"io/ioutil"
	"os"
	"path"
	"strings"

	"github.com/OpenBazaar/jsonpb"
	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/golang/protobuf/proto"
)

const (
	PostTitleMaxCharacters		= 280
	PostLongFormMaxCharacters	= 50000
)

type postData struct {
	Hash      string    `json:"hash"`
	Slug      string    `json:"slug"`
	Title     string    `json:"title"`
	Thumbnail thumbnail `json:"thumbnail"`
	Reference reference `json:"reference"`
	Timestamp string		`json:"timestamp"`
}

type reference struct {
	PeerId    string		`json:"peerId"`
}

// Add our identity to the post and sign it
func (n *OpenBazaarNode) SignPost(post *pb.Post) (*pb.SignedPost, error) {

	sl := new(pb.SignedPost)

	// Add the vendor ID to the post
	id := new(pb.ID)
	id.PeerID = n.IpfsNode.Identity.Pretty()
	pubkey, err := n.IpfsNode.PrivateKey.GetPublic().Bytes()
	if err != nil {
		return sl, err
	}
	profile, err := n.GetProfile()
	if err == nil {
		id.Handle = profile.Handle
	}
	p := new(pb.ID_Pubkeys)
	p.Identity = pubkey
	ecPubKey, err := n.Wallet.MasterPublicKey().ECPubKey()
	if err != nil {
		return sl, err
	}
	p.Bitcoin = ecPubKey.SerializeCompressed()
	id.Pubkeys = p
	post.VendorID = id

	// Sign the GUID with the Bitcoin key
	ecPrivKey, err := n.Wallet.MasterPrivateKey().ECPrivKey()
	if err != nil {
		return sl, err
	}
	sig, err := ecPrivKey.Sign([]byte(id.PeerID))
	id.BitcoinSig = sig.Serialize()

	// Sign post
	serializedPost, err := proto.Marshal(post)
	if err != nil {
		return sl, err
	}
	idSig, err := n.IpfsNode.PrivateKey.Sign(serializedPost)
	if err != nil {
		return sl, err
	}
	sl.Post = post
	sl.Signature = idSig
	return sl, nil
}

func (n *OpenBazaarNode) UpdatePostIndex(post *pb.SignedPost) error {
	ld, err := n.extractpostData(post)
	if err != nil {
		return err
	}
	index, err := n.getPostIndex()
	if err != nil {
		return err
	}
	return n.updatePostOnDisk(index, ld, false)
}

func (n *OpenBazaarNode) extractpostData(post *pb.SignedPost) (postData, error) {
	postPath := path.Join(n.RepoPath, "root", "posts", post.Post.Slug+".json")

	postHash, err := ipfs.GetHashOfFile(n.Context, postPath)
	if err != nil {
		return postData{}, err
	}

	ld := postData{
		Hash:      postHash,
		Slug:      post.Post.Slug,
		Title:     post.Post.Title,
		Reference: reference{post.Post.Reference.PeerId},
	}

	if post.Post.Timestamp != nil {
		ld.Timestamp = FormatRFC3339PB(*post.Post.Timestamp)
	}

	if len(post.Post.Images) > 0 {
			ld.Thumbnail = thumbnail{
					post.Post.Images[0].Tiny,
					post.Post.Images[0].Small,
					post.Post.Images[0].Medium,
			}
	}
	return ld, nil
}

func (n *OpenBazaarNode) getPostIndex() ([]postData, error) {
	indexPath := path.Join(n.RepoPath, "root", "posts.json")

	var index []postData

	_, ferr := os.Stat(indexPath)
	if !os.IsNotExist(ferr) {
		// Read existing file
		file, err := ioutil.ReadFile(indexPath)
		if err != nil {
			return index, err
		}
		err = json.Unmarshal(file, &index)
		if err != nil {
			return index, err
		}
	}
	return index, nil
}

// Update the posts.json file in the posts directory
func (n *OpenBazaarNode) updatePostOnDisk(index []postData, ld postData, updateRatings bool) error {
	indexPath := path.Join(n.RepoPath, "root", "posts.json")
	// Check to see if the post we are adding already exists in the list. If so delete it.
	for i, d := range index {
		if d.Slug != ld.Slug {
			continue
		}

		if len(index) == 1 {
			index = []postData{}
			break
		}
		index = append(index[:i], index[i+1:]...)
	}

	// Append our post with the new hash to the list
	index = append(index, ld)

	// Write it back to file
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

// Update the hashes in the posts.json file
func (n *OpenBazaarNode) UpdatePostHashes(hashes map[string]string) error {
	indexPath := path.Join(n.RepoPath, "root", "posts.json")

	var index []postData

	_, ferr := os.Stat(indexPath)
	if os.IsNotExist(ferr) {
		return nil
	}
	// Read existing file
	file, err := ioutil.ReadFile(indexPath)
	if err != nil {
		return err
	}
	err = json.Unmarshal(file, &index)
	if err != nil {
		return err
	}

	// Update hashes
	for _, d := range index {
		hash, ok := hashes[d.Slug]
		if ok {
			d.Hash = hash
		}
	}

	// Write it back to file
	f, err := os.Create(indexPath)
	defer f.Close()
	if err != nil {
		return err
	}

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

// Return the current number of posts
func (n *OpenBazaarNode) GetPostCount() int {
	indexPath := path.Join(n.RepoPath, "root", "posts.json")

	// Read existing file
	file, err := ioutil.ReadFile(indexPath)
	if err != nil {
		return 0
	}

	var index []postData
	err = json.Unmarshal(file, &index)
	if err != nil {
		return 0
	}
	return len(index)
}

// Deletes the post directory, and removes the post from the index
func (n *OpenBazaarNode) DeletePost(slug string) error {
	toDelete := path.Join(n.RepoPath, "root", "posts", slug+".json")
	err := os.Remove(toDelete)
	if err != nil {
		return err
	}
	var index []postData
	indexPath := path.Join(n.RepoPath, "root", "posts.json")
	_, ferr := os.Stat(indexPath)
	if !os.IsNotExist(ferr) {
		// Read existing file
		file, err := ioutil.ReadFile(indexPath)
		if err != nil {
			return err
		}
		err = json.Unmarshal(file, &index)
		if err != nil {
			return err
		}
	}

	// Check to see if the slug exists in the list. If so delete it.
	for i, d := range index {
		if d.Slug != slug {
			continue
		}

		if len(index) == 1 {
			index = []postData{}
			break
		}
		index = append(index[:i], index[i+1:]...)
	}

	// Write the index back to file
	f, err := os.Create(indexPath)
	defer f.Close()
	if err != nil {
		return err
	}

	j, jerr := json.MarshalIndent(index, "", "    ")
	if jerr != nil {
		return jerr
	}
	_, werr := f.Write(j)
	if werr != nil {
		return werr
	}

	return n.updateProfileCounts()
}

func (n *OpenBazaarNode) GetPosts() ([]byte, error) {
	indexPath := path.Join(n.RepoPath, "root", "posts.json")
	file, err := ioutil.ReadFile(indexPath)
	if os.IsNotExist(err) {
		return []byte("[]"), nil
	} else if err != nil {
		return nil, err
	}

	// Unmarshal the index to check if file contains valid json
	var index []postData
	err = json.Unmarshal(file, &index)
	if err != nil {
		return nil, err
	}

	// Return bytes read from file
	return file, nil
}

func (n *OpenBazaarNode) GetPostFromHash(hash string) (*pb.SignedPost, error) {
	// Read posts.json
	indexPath := path.Join(n.RepoPath, "root", "posts.json")
	file, err := ioutil.ReadFile(indexPath)
	if err != nil {
		return nil, err
	}

	// Unmarshal the index
	var index []postData
	err = json.Unmarshal(file, &index)
	if err != nil {
		return nil, err
	}

	// Extract slug that matches hash
	var slug string
	for _, data := range index {
		if data.Hash == hash {
			slug = data.Slug
			break
		}
	}

	if slug == "" {
		return nil, errors.New("Post does not exist")
	}
	return n.GetPostFromSlug(slug)
}

func (n *OpenBazaarNode) GetPostFromSlug(slug string) (*pb.SignedPost, error) {
	// Read post file
	postPath := path.Join(n.RepoPath, "root", "posts", slug+".json")
	file, err := ioutil.ReadFile(postPath)
	if err != nil {
		return nil, err
	}

	// Unmarshal post
	sl := new(pb.SignedPost)
	err = jsonpb.UnmarshalString(string(file), sl)
	if err != nil {
		return nil, err
	}

	return sl, nil
}

/* Performs a ton of checks to make sure the posts is formatted correctly. We should not allow
   invalid posts to be saved. This function needs to be maintained in conjunction with contracts.proto */
func validatePost(post *pb.Post) (err error) {
	defer func() {
		if r := recover(); r != nil {
			switch x := r.(type) {
			case string:
				err = errors.New(x)
			case error:
				err = x
			default:
				err = errors.New("Unknown panic")
			}
		}
	}()

	// Slug
	if post.Slug == "" {
		return errors.New("Slug must not be empty")
	}
	if len(post.Slug) > SentenceMaxCharacters {
		return fmt.Errorf("Slug is longer than the max of %d", SentenceMaxCharacters)
	}
	if strings.Contains(post.Slug, " ") {
		return errors.New("Slugs cannot contain spaces")
	}
	if strings.Contains(post.Slug, "/") {
		return errors.New("Slugs cannot contain file separators")
	}

	// Tile
	if post.Title == "" {
		return errors.New("Post must have a title")
	}
	if len(post.Title) > PostTitleMaxCharacters {
		return fmt.Errorf("Title is longer than the max of %d", PostTitleMaxCharacters)
	}

	// Long Form
	if len(post.LongForm) > PostLongFormMaxCharacters {
		return fmt.Errorf("Post is longer than the max of %d characters", PostLongFormMaxCharacters)
	}

	// Images
	if len(post.Images) > MaxListItems {
		return fmt.Errorf("Number of post images is greater than the max of %d", MaxListItems)
	}
	for _, img := range post.Images {
		_, err := cid.Decode(img.Tiny)
		if err != nil {
			return errors.New("Tiny image hashes must be properly formatted CID")
		}
		_, err = cid.Decode(img.Small)
		if err != nil {
			return errors.New("Small image hashes must be properly formatted CID")
		}
		_, err = cid.Decode(img.Medium)
		if err != nil {
			return errors.New("Medium image hashes must be properly formatted CID")
		}
		_, err = cid.Decode(img.Large)
		if err != nil {
			return errors.New("Large image hashes must be properly formatted CID")
		}
		_, err = cid.Decode(img.Original)
		if err != nil {
			return errors.New("Original image hashes must be properly formatted CID")
		}
		if img.Filename == "" {
			return errors.New("Image file names must not be nil")
		}
		if len(img.Filename) > FilenameMaxCharacters {
			return fmt.Errorf("Image filename length must be less than the max of %d", FilenameMaxCharacters)
		}
	}

	return nil
}

func verifySignaturesOnPost(sl *pb.SignedPost) error {
	// Verify identity signature on the post
	if err := verifySignature(
		sl.Post,
		sl.Post.VendorID.Pubkeys.Identity,
		sl.Signature,
		sl.Post.VendorID.PeerID,
	); err != nil {
		switch err.(type) {
		case noSigError:
			return errors.New("Post does not contain signature")
		case invalidSigError:
			return errors.New("Vendor's identity signature on post failed to verify")
		case matchKeyError:
			return errors.New("Public key in order does not match reported buyer ID")
		default:
			return err
		}
	}

	// Verify the bitcoin signature in the ID
	if err := verifyBitcoinSignature(
		sl.Post.VendorID.Pubkeys.Bitcoin,
		sl.Post.VendorID.BitcoinSig,
		sl.Post.VendorID.PeerID,
	); err != nil {
		switch err.(type) {
		case invalidSigError:
			return errors.New("Vendor's bitcoin signature on GUID failed to verify")
		default:
			return err
		}
	}
	return nil
}