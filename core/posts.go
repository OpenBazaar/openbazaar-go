package core

import (
	"encoding/json"
	"errors"
	"fmt"
	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	"io/ioutil"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/OpenBazaar/jsonpb"
	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/golang/protobuf/proto"
	"github.com/kennygrant/sanitize"
)

// Constants for validation
const (
	PostTitleMaxCharacters    = 280
	PostLongFormMaxCharacters = 50000
	MaxPostTags               = 50
	PostTagsMaxCharacters     = 80
)

// JSON structure returned for each post from GETPosts
type postData struct {
	Hash      string      `json:"hash"`
	Slug      string      `json:"slug"`
	Title     string      `json:"title"`
	Images    []postImage `json:"images"`
	Tags      []string    `json:"tags"`
	Timestamp string      `json:"timestamp"`
}

type postImage struct {
	Tiny   string `json:"tiny"`
	Small  string `json:"small"`
	Medium string `json:"medium"`
}

//GeneratePostSlug  [Create a slug for the post based on the title, if a slug is missing]
func (n *OpenBazaarNode) GeneratePostSlug(title string) (string, error) {
	title = strings.Replace(title, "/", "", -1)
	slugFromTitle := func(title string) string {
		l := SentenceMaxCharacters - SlugBuffer
		if len(title) < SentenceMaxCharacters-SlugBuffer {
			l = len(title)
		}
		return url.QueryEscape(sanitize.Path(strings.ToLower(title[:l])))
	}
	counter := 1
	slugBase := slugFromTitle(title)
	slugToTry := slugBase
	for {
		_, err := n.GetPostFromSlug(slugToTry)
		if os.IsNotExist(err) {
			return slugToTry, nil
		} else if err != nil {
			return "", err
		}
		slugToTry = slugBase + strconv.Itoa(counter)
		counter++
	}
}

//SignPost  [Add the peer's identity to the post and sign it]
func (n *OpenBazaarNode) SignPost(post *pb.Post) (*pb.SignedPost, error) {

	sp := new(pb.SignedPost)

	// Check the post data is correct for continuing
	if err := validatePost(post); err != nil {
		return sp, err
	}

	// Add the vendor ID to the post
	id := new(pb.ID)
	id.PeerID = n.IpfsNode.Identity.Pretty()
	pubkey, err := n.IpfsNode.PrivateKey.GetPublic().Bytes()
	if err != nil {
		return sp, err
	}
	profile, err := n.GetProfile()
	if err == nil {
		id.Handle = profile.Handle
	}
	p := new(pb.ID_Pubkeys)
	p.Identity = pubkey
	ecPubKey, err := n.Wallet.MasterPublicKey().ECPubKey()
	if err != nil {
		return sp, err
	}
	p.Bitcoin = ecPubKey.SerializeCompressed()
	id.Pubkeys = p
	post.VendorID = id

	// Sign the GUID with the Bitcoin key
	ecPrivKey, err := n.Wallet.MasterPrivateKey().ECPrivKey()
	if err != nil {
		return sp, err
	}
	sig, err := ecPrivKey.Sign([]byte(id.PeerID))
	if err != nil {
		return sp, err
	}
	id.BitcoinSig = sig.Serialize()

	// Sign post
	serializedPost, err := proto.Marshal(post)
	if err != nil {
		return sp, err
	}
	idSig, err := n.IpfsNode.PrivateKey.Sign(serializedPost)
	if err != nil {
		return sp, err
	}
	sp.Post = post
	sp.Signature = idSig
	return sp, nil
}

//UpdatePostIndex  [Update the posts index]
func (n *OpenBazaarNode) UpdatePostIndex(post *pb.SignedPost) error {
	ld, err := n.extractpostData(post)
	if err != nil {
		return err
	}
	index, err := n.getPostIndex()
	if err != nil {
		return err
	}
	return n.updatePostOnDisk(index, ld)
}

//extractpostData  [Extract data from the post, used to make postData and in GETPosts]
func (n *OpenBazaarNode) extractpostData(post *pb.SignedPost) (postData, error) {
	postPath := path.Join(n.RepoPath, "root", "posts", post.Post.Slug+".json")

	// Get the hash of the post's file and add to postHash variable
	postHash, err := ipfs.GetHashOfFile(n.IpfsNode, postPath)
	if err != nil {
		return postData{}, err
	}

	/* Generic function to loop through each element in an array
	and check if a certain string-type variable exists */
	contains := func(s []string, e string) bool {
		for _, a := range s {
			if a == e {
				return true
			}
		}
		return false
	}

	/* Add a tag in the post to an array called tags,
	which will be added to the postData object below */
	tags := []string{}
	for _, tag := range post.Post.Tags {
		if !contains(tags, tag) {
			tags = append(tags, tag)
		}
		if len(tags) > 15 {
			tags = tags[0:15]
		}
	}

	// Create the postData object
	ld := postData{
		Hash:  postHash,
		Slug:  post.Post.Slug,
		Title: post.Post.Title,
		Tags:  tags,
	}

	// Add a timestamp to postData if it doesn't exist
	if post.Post.Timestamp != nil {
		ld.Timestamp = FormatRFC3339PB(*post.Post.Timestamp)
	}

	// Add images to postData if they exist
	imageArray := []postImage{}
	if len(post.Post.Images) > 0 {
		for _, imageSlice := range post.Post.Images {
			imageObject := postImage{imageSlice.Tiny, imageSlice.Small, imageSlice.Medium}
			imageArray = append(imageArray, imageObject)
		}
		if len(imageArray) > 8 {
			imageArray = imageArray[0:8]
		}
	}
	ld.Images = imageArray

	// Returns postData in its final form
	return ld, nil
}

//getPostIndex  [Get the post's index]
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

//updatePostOnDisk  [Update the posts.json file in the posts directory]
func (n *OpenBazaarNode) updatePostOnDisk(index []postData, ld postData) error {
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

//UpdatePostHashes  [Update the hashes in the posts.json file]
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

//GetPostCount  [Return the current number of posts]
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

//DeletePost  [Deletes the post directory, and removes the post from the index]
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

	return n.updateProfileCounts()
}

//GetPosts  [Get a list of the posts]
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

//GetPostFromHash  [Get a post based on the hash]
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

//GetPostFromSlug  [Get a post based on the slug]
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

//validatePost  [Validate the post]
/* Performs a ton of checks to make sure the posts is formatted correctly. We should not allow
   invalid posts to be saved. This function needs to be maintained in conjunction with posts.proto */
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

	// Tags
	if len(post.Tags) > MaxPostTags {
		return fmt.Errorf("Tags in the post is longer than the max of %d characters", MaxPostTags)
	}
	for _, tag := range post.Tags {
		if tag == "" {
			return errors.New("Tags must not be empty")
		}
		if len(tag) > PostTagsMaxCharacters {
			return fmt.Errorf("Tags must be less than max of %d", PostTagsMaxCharacters)
		}
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
