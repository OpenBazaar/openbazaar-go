package core

import (
	"encoding/json"
	"errors"
	"fmt"

	cid "gx/ipfs/QmTbxNB1NwDesLmKTscr4udL2tVP7MaxvXnD1D9yX7g3PN/go-cid"

	"io/ioutil"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/OpenBazaar/jsonpb"
	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/golang/protobuf/proto"
)

// Constants for validation
const (
	// PostStatusMaxCharacters - Maximum length of the status field of a post
	PostStatusMaxCharacters = 280
	// PostLongFormMaxCharacters - Maximum length of the longForm field of a post
	PostLongFormMaxCharacters = 50000
	// PostMaximumTotalTags - Maximum number of tags a post can have
	PostMaximumTotalTags = 50
	// PostMaximumTotalChannels - Maximum number of channels a post can be addressed to
	PostMaximumTotalChannels = 30
	// PostTagsMaxCharacters - Maximum character length of a tag
	PostTagsMaxCharacters = 256
	// PostChannelsMaxCharacters - Maximum character length of a channel
	PostChannelsMaxCharacters = 256
	// PostReferenceMaxCharacters - Maximum character length of a reference
	PostReferenceMaxCharacters = 256
)

// Errors
var (
	// ErrPostUnknownValidationPanic - post has an unknown panic error
	ErrPostUnknownValidationPanic = errors.New("unexpected validation panic")

	// ErrPostSlugNotEmpty - post slug is empty error
	ErrPostSlugNotEmpty = errors.New("slug must not be empty")

	// ErrPostSlugTooLong - post slug longer than max characters error
	ErrPostSlugTooLong = fmt.Errorf("slug is longer than the max of %d", repo.SentenceMaxCharacters)

	// ErrPostSlugContainsSpaces - post slug has spaces error
	ErrPostSlugContainsSpaces = errors.New("slugs cannot contain spaces")

	// ErrPostSlugContainsSlashes - post slug has file separators
	ErrPostSlugContainsSlashes = errors.New("slugs cannot contain file separators")

	// ErrPostInvalidType - post type is invalid error
	ErrPostInvalidType = errors.New("invalid post type")

	// ErrPostStatusTooLong - post 'status' is longer than max characters error
	ErrPostStatusTooLong = fmt.Errorf("status is longer than the max of %d", PostStatusMaxCharacters)

	// ErrPostBodyTooLong - post 'longForm' is longer than max characters error
	ErrPostBodyTooLong = fmt.Errorf("post is longer than the max of %d characters", PostLongFormMaxCharacters)

	// ErrPostTagsTooMany - post tags longer than max length error
	ErrPostTagsTooMany = fmt.Errorf("tags in the post is longer than the max of %d", PostMaximumTotalTags)

	// ErrPostTagsEmpty - post has empty tags error
	ErrPostTagsEmpty = errors.New("tags must not be empty")

	// ErrPostTagTooLong - post tag has characters longer than max length error
	ErrPostTagTooLong = fmt.Errorf("tags must be less than max of %d characters", PostTagsMaxCharacters)

	// ErrPostChannelsTooMany - post channels longer than max length error
	ErrPostChannelsTooMany = fmt.Errorf("channels in the post is longer than the max of %d", PostMaximumTotalChannels)

	// ErrPostChannelTooLong - post channel has characters longer than max length error
	ErrPostChannelTooLong = fmt.Errorf("channels must be less than max of %d characters", PostChannelsMaxCharacters)

	// ErrPostReferenceEmpty - post has an empty reference error
	ErrPostReferenceEmpty = errors.New("reference must not be empty")

	// ErrPostReferenceTooLong - post reference has characters longer than max length error
	ErrPostReferenceTooLong = fmt.Errorf("reference is longer than the max of %d", PostReferenceMaxCharacters)

	// ErrPostReferenceContainsSpaces - post reference has spaces error
	ErrPostReferenceContainsSpaces = errors.New("reference cannot contain spaces")

	// ErrPostImagesTooMany - post images longer than max error
	ErrPostImagesTooMany = fmt.Errorf("number of post images is greater than the max of %d", repo.MaxListItems)

	// ErrPostImageTinyFormatInvalid - post tiny image hash incorrectly formatted error
	ErrPostImageTinyFormatInvalid = errors.New("tiny image hashes must be properly formatted CID")

	// ErrPostImageSmallFormatInvalid - post small image hash incorrectly formatted error
	ErrPostImageSmallFormatInvalid = errors.New("small image hashes must be properly formatted CID")

	// ErrPostImageMediumFormatInvalid - post medium image hash incorrectly formatted error
	ErrPostImageMediumFormatInvalid = errors.New("medium image hashes must be properly formatted CID")

	// ErrPostImageLargeFInvalidormat - post large image hash incorrectly formatted error
	ErrPostImageLargeFormatInvalid = errors.New("large image hashes must be properly formatted CID")

	// ErrPostImageOriginalFormatInvalid - post original image hash incorrectly formatted error
	ErrPostImageOriginalFormatInvalid = errors.New("original image hashes must be properly formatted CID")

	// ErrPostImageFilenameNil - post image filename is nil error
	ErrPostImageFilenameNil = errors.New("image file names must not be nil")

	// ErrPostImageFilenameTooLong - post image filename length longer than max
	ErrPostImageFilenameTooLong = fmt.Errorf("image filename length must be less than the max of %d", repo.FilenameMaxCharacters)
)

// JSON structure returned for each post from GETPosts
type postData struct {
	Hash      string      `json:"hash"`
	Slug      string      `json:"slug"`
	PostType  string      `json:"postType"`
	Status    string      `json:"status"`
	Images    []postImage `json:"images"`
	Tags      []string    `json:"tags"`
	Channels  []string    `json:"channels"`
	Reference string      `json:"reference"`
	Timestamp string      `json:"timestamp"`
}

type postImage struct {
	Tiny   string `json:"tiny"`
	Small  string `json:"small"`
	Medium string `json:"medium"`
}

//GeneratePostSlug  [Create a slug for the post based on the status, if a slug is missing]
func (n *OpenBazaarNode) GeneratePostSlug(status string) (string, error) {
	status = strings.Replace(status, "/", "", -1)
	counter := 1
	slugBase := repo.CreateSlugFor(status)
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
	ecPubKey, err := n.MasterPrivateKey.ECPubKey()
	if err != nil {
		return sp, err
	}
	p.Bitcoin = ecPubKey.SerializeCompressed()
	id.Pubkeys = p
	post.VendorID = id

	// Sign the GUID with the Bitcoin key
	ecPrivKey, err := n.MasterPrivateKey.ECPrivKey()
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

	// Create the postData object
	ld := postData{
		Hash:      postHash,
		Slug:      post.Post.Slug,
		PostType:  post.Post.PostType.String(),
		Status:    post.Post.Status,
		Tags:      makeUnique(post.Post.Tags, 15),
		Channels:  makeUnique(post.Post.Channels, 15),
		Reference: post.Post.Reference,
	}

	// Add a timestamp to postData if it doesn't exist
	if post.Post.Timestamp != nil {
		ld.Timestamp = FormatRFC3339PB(*post.Post.Timestamp)
	}

	// Add images to postData if they exist
	var imageArray []postImage
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

// makeUnique returns a new slice of unique strings based on src which is not mutated
func makeUnique(src []string, maxLength int) []string {
	result := make([]string, 0, maxLength)
	uniqueMap := make(map[string]struct{}, maxLength)
	for _, v := range src {
		if _, ok := uniqueMap[v]; ok {
			continue
		}
		uniqueMap[v] = struct{}{}

		result = append(result, v)
		if len(result) >= maxLength {
			break
		}
	}
	return result
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
		return nil, errors.New("post does not exist")
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
				err = ErrPostUnknownValidationPanic
			}
		}
	}()

	// Slug
	if post.Slug == "" {
		return ErrPostSlugNotEmpty
	}
	if len(post.Slug) > repo.SentenceMaxCharacters {
		return ErrPostSlugTooLong
	}
	if strings.Contains(post.Slug, " ") {
		return ErrPostSlugContainsSpaces
	}
	if strings.Contains(post.Slug, "/") {
		return ErrPostSlugContainsSlashes
	}

	// Type
	if _, ok := pb.Post_PostType_value[post.PostType.String()]; !ok {
		return ErrPostInvalidType
	}

	// Status
	if len(post.Status) > PostStatusMaxCharacters {
		return ErrPostStatusTooLong
	}

	// Long Form
	if len(post.LongForm) > PostLongFormMaxCharacters {
		return ErrPostBodyTooLong
	}

	// Tags
	if len(post.Tags) > PostMaximumTotalTags {
		return ErrPostTagsTooMany
	}
	for _, tag := range post.Tags {
		if tag == "" {
			return ErrPostTagsEmpty
		}
		if len(tag) > PostTagsMaxCharacters {
			return ErrPostTagTooLong
		}
	}

	// Channels
	if len(post.Channels) > PostMaximumTotalChannels {
		return ErrPostChannelsTooMany
	}
	for _, channel := range post.Channels {
		if len(channel) > PostChannelsMaxCharacters {
			return ErrPostChannelTooLong
		}
	}

	// Reference
	if post.PostType == pb.Post_COMMENT || post.PostType == pb.Post_REPOST {
		if post.Reference == "" {
			return ErrPostReferenceEmpty
		}
		if len(post.Reference) > PostReferenceMaxCharacters {
			return ErrPostReferenceTooLong
		}
		if strings.Contains(post.Reference, " ") {
			return ErrPostReferenceContainsSpaces
		}
	}

	// Images
	if len(post.Images) > repo.MaxListItems {
		return ErrPostImagesTooMany
	}
	for _, img := range post.Images {
		_, err := cid.Decode(img.Tiny)
		if err != nil {
			return ErrPostImageTinyFormatInvalid
		}
		_, err = cid.Decode(img.Small)
		if err != nil {
			return ErrPostImageSmallFormatInvalid
		}
		_, err = cid.Decode(img.Medium)
		if err != nil {
			return ErrPostImageMediumFormatInvalid
		}
		_, err = cid.Decode(img.Large)
		if err != nil {
			return ErrPostImageLargeFormatInvalid
		}
		_, err = cid.Decode(img.Original)
		if err != nil {
			return ErrPostImageOriginalFormatInvalid
		}
		if img.Filename == "" {
			return ErrPostImageFilenameNil
		}
		if len(img.Filename) > repo.FilenameMaxCharacters {
			return ErrPostImageFilenameTooLong
		}
	}

	return nil
}
