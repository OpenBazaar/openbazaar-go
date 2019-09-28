package migrations

//import (
//"context"
//"encoding/json"
//"fmt"
//"io/ioutil"
//"os"
//"path"
//"strconv"
//"time"

//crypto "gx/ipfs/QmTW4SdgBWq9GjsBsHeUx8WuGxzhgzAf88UMH2w62PC8yK/go-libp2p-crypto"

//"github.com/OpenBazaar/jsonpb"
//"github.com/OpenBazaar/openbazaar-go/ipfs"
//"github.com/OpenBazaar/openbazaar-go/pb"
//"github.com/golang/protobuf/proto"
//timestamp "github.com/golang/protobuf/ptypes/timestamp"
//ipfscore "github.com/ipfs/go-ipfs/core"
//"github.com/ipfs/go-ipfs/repo/fsrepo"
//)

type Migration027 struct{}

func (Migration027) Up(a, b string, c bool) error   { return nil }
func (Migration027) Down(a, b string, c bool) error { return nil }

//type price0 struct {
//CurrencyCode string  `json:"currencyCode"`
//Amount       string  `json:"amount"`
//Modifier     float32 `json:"modifier"`
//}

//[>
//type thumbnail struct {
//Tiny   string `json:"tiny"`
//Small  string `json:"small"`
//Medium string `json:"medium"`
//}

//type ListingData struct {
//Hash               string    `json:"hash"`
//Slug               string    `json:"slug"`
//Title              string    `json:"title"`
//Categories         []string  `json:"categories"`
//NSFW               bool      `json:"nsfw"`
//ContractType       string    `json:"contractType"`
//Description        string    `json:"description"`
//Thumbnail          thumbnail `json:"thumbnail"`
//Price              price     `json:"price"`
//ShipsTo            []string  `json:"shipsTo"`
//FreeShipping       []string  `json:"freeShipping"`
//Language           string    `json:"language"`
//AverageRating      float32   `json:"averageRating"`
//RatingCount        uint32    `json:"ratingCount"`
//ModeratorIDs       []string  `json:"moderators"`
//AcceptedCurrencies []string  `json:"acceptedCurrencies"`
//CoinType           string    `json:"coinType"`
//}
//*/

//type m27_filterListing struct {
//Hash string `json:"hash"`
//Slug string `json:"slug"`
//}

//type m27_ListingData struct {
//Hash         string   `json:"hash"`
//Slug         string   `json:"slug"`
//Title        string   `json:"title"`
//Categories   []string `json:"categories"`
//NSFW         bool     `json:"nsfw"`
//ContractType string   `json:"contractType"`
//Description  string   `json:"description"`
//Thumbnail    struct {
//Tiny   string `json:"tiny"`
//Small  string `json:"small"`
//Medium string `json:"medium"`
//} `json:"thumbnail"`
//Price struct {
//CurrencyCode string  `json:"currencyCode"`
//Modifier     float32 `json:"modifier"`
//} `json:"price"`
//ShipsTo            []string `json:"shipsTo"`
//FreeShipping       []string `json:"freeShipping"`
//Language           string   `json:"language"`
//AverageRating      float32  `json:"averageRating"`
//RatingCount        uint32   `json:"ratingCount"`
//ModeratorIDs       []string `json:"moderators"`
//AcceptedCurrencies []string `json:"acceptedCurrencies"`
//CoinType           string   `json:"coinType"`
//}

//type m27_ListingDatav5 struct {
//Hash         string   `json:"hash"`
//Slug         string   `json:"slug"`
//Title        string   `json:"title"`
//Categories   []string `json:"categories"`
//NSFW         bool     `json:"nsfw"`
//ContractType string   `json:"contractType"`
//Description  string   `json:"description"`
//Thumbnail    struct {
//Tiny   string `json:"tiny"`
//Small  string `json:"small"`
//Medium string `json:"medium"`
//} `json:"thumbnail"`
//Price              price0   `json:"price"`
//ShipsTo            []string `json:"shipsTo"`
//FreeShipping       []string `json:"freeShipping"`
//Language           string   `json:"language"`
//AverageRating      float32  `json:"averageRating"`
//RatingCount        uint32   `json:"ratingCount"`
//ModeratorIDs       []string `json:"moderators"`
//AcceptedCurrencies []string `json:"acceptedCurrencies"`
//CoinType           string   `json:"coinType"`
//}

//type m27_Listing struct {
//Slug     string  `json:"slug,omitempty"`
//VendorID *m27_ID `json:"vendorID,omitempty"`
//Metadata *struct {
//Version            uint32   `json:"version,omitempty"`
//ContractType       string   `json:"contractType,omitempty"`
//Format             string   `json:"format,omitempty"`
//Expiry             string   `json:"expiry,omitempty"`
//AcceptedCurrencies []string `json:"acceptedCurrencies,omitempty"`
//PricingCurrency    string   `json:"pricingCurrency,omitempty"`
//Language           string   `json:"language,omitempty"`
//EscrowTimeoutHours uint32   `json:"escrowTimeoutHours,omitempty"`
//CoinType           string   `json:"coinType,omitempty"`
//CoinDivisibility   uint32   `json:"coinDivisibility,omitempty"`
//PriceModifier      float32  `json:"priceModifier,omitempty"`
//} `json:"metadata,omitempty"`
//Item               *m27_ListingItem             `json:"item,omitempty"`
//ShippingOptions    []*m27_ListingShippingOption `json:"shippingOptions,omitempty"`
//Taxes              []*m27_Listing_Tax           `json:"taxes,omitempty"`
//Coupons            []*m27_Listing_Coupon        `json:"coupons,omitempty"`
//Moderators         []string                     `json:"moderators,omitempty"`
//TermsAndConditions string                       `json:"termsAndConditions,omitempty"`
//RefundPolicy       string
//}

//type m27_ID struct {
//PeerID  string `json:"peerID,omitempty"`
//Handle  string `json:"handle,omitempty"`
//Pubkeys *struct {
//Identity []byte `json:"identity,omitempty"`
//Bitcoin  []byte `json:"bitcoin,omitempty"`
//} `json:"pubkeys,omitempty"`
//BitcoinSig []byte `json:"bitcoinSig,omitempty"`
//}

//type m27_ListingItem struct {
//Title          string                    `json:"title,omitempty"`
//Description    string                    `json:"description,omitempty"`
//ProcessingTime string                    `json:"processingTime,omitempty"`
//Price          uint64                    `json:"price,omitempty"`
//Nsfw           bool                      `json:"nsfw,omitempty"`
//Tags           []string                  `json:"tags,omitempty"`
//Images         []*pb.Listing_Item_Image  `json:"images,omitempty"`
//Categories     []string                  `json:"categories,omitempty"`
//Grams          float32                   `json:"grams,omitempty"`
//Condition      string                    `json:"condition,omitempty"`
//Options        []*pb.Listing_Item_Option `json:"options,omitempty"`
//Skus           []*m27_Listing_Item_Sku   `json:"skus,omitempty"`
//}

//[>
//type m27_Listing_Item_Option struct {
//Name        string                            `json:"name,omitempty"`
//Description string                            `json:"description,omitempty"`
//Variants    []*pb.Listing_Item_Option_Variant `json:"variants,omitempty"`
//}

//type m27_Listing_Item_Option_Variant struct {
//Name  string                 `json:"name,omitempty"`
//Image *pb.Listing_Item_Image `json:"image,omitempty"`
//}
//*/

//type m27_Listing_Item_Sku struct {
//VariantCombo []uint32 `json:"variantCombo,omitempty"`
//ProductID    string   `json:"productID,omitempty"`
//Surcharge    int64    `json:"surcharge,omitempty"`
//Quantity     int64    `json:"quantity,omitempty"`
//}

//[>
//type m27_Listing_Item_Image struct {
//Filename string `json:"filename,omitempty"`
//Original string `json:"original,omitempty"`
//Large    string `json:"large,omitempty"`
//Medium   string `json:"medium,omitempty"`
//Small    string `json:"small,omitempty"`
//Tiny     string `json:"tiny,omitempty"`
//}
//*/

//type m27_ListingShippingOption struct {
//Name     string                                `json:"name,omitempty"`
//Type     string                                `json:"type,omitempty"`
//Regions  []string                              `json:"regions,omitempty"`
//Services []*m27_Listing_ShippingOption_Service `json:"services,omitempty"`
//}

//type m27_Listing_ShippingOption_Service struct {
//Name                string `json:"name,omitempty"`
//Price               uint64 `json:"price,omitempty"`
//EstimatedDelivery   string `json:"estimatedDelivery,omitempty"`
//AdditionalItemPrice uint64 `json:"additionalItemPrice,omitempty"`
//}

//type m27_Listing_Tax struct {
//TaxType     string   `json:"taxType,omitempty"`
//TaxRegions  []string `json:"taxRegions,omitempty"`
//TaxShipping bool     `json:"taxShipping,omitempty"`
//Percentage  float32  `json:"percentage,omitempty"`
//}

//type m27_Listing_Coupon struct {
//PercentDiscount float32 `json:"percentDiscount,omitempty"`
//PriceDiscount   uint64  `json:"priceDiscount,omitempty"`
//Title           string  `json:"title"`
//DiscountCode    string  `json:"discountCode"`
//Hash            string  `json:"hash"`
//}

//type m27_SignedListing struct {
//Listing   *m27_Listing `json:"listing"`
//Hash      string       `json:"hash"`
//Signature []byte       `json:"signature"`
//}

//func (m *m27_SignedListing) Reset()         { *m = m27_SignedListing{} }
//func (m *m27_SignedListing) String() string { return proto.CompactTextString(m) }
//func (*m27_SignedListing) ProtoMessage()    {}

//type m27_ListingFilter struct {
//Slug     string                      `json:"slug,omitempty"`
//Metadata *m27_Listing_MetadataFilter `json:"metadata,omitempty"`
//}

//type m27_Listing_MetadataFilter struct {
//Version uint32 `json:"version,omitempty"`
//}

//type m27_SignedListingDataFilter struct {
//Listing   *m27_ListingFilter `json:"listing"`
//Hash      string             `json:"hash"`
//Signature []byte             `json:"signature"`
//}

//func m27_GetIdentityKey(repoPath, databasePassword string, testnetEnabled bool) ([]byte, error) {
//db, err := OpenDB(repoPath, databasePassword, testnetEnabled)
//if err != nil {
//return nil, err
//}
//defer db.Close()

//var identityKey []byte
//err = db.
//QueryRow("select value from config where key=?", "identityKey").
//Scan(&identityKey)
//if err != nil {
//return nil, err
//}
//return identityKey, nil
//}

//func (Migration027) Up(repoPath, databasePassword string, testnetEnabled bool) error {
//listingsIndexFilePath := path.Join(repoPath, "root", "listings.json")

//// Find all crypto listings
//if _, err := os.Stat(listingsIndexFilePath); os.IsNotExist(err) {
//// Finish early if no listings are found
//return writeRepoVer(repoPath, 28)
//}
//listingsIndexJSONBytes, err := ioutil.ReadFile(listingsIndexFilePath)
//if err != nil {
//return err
//}

//var listingsIndex []interface{}
//err = json.Unmarshal(listingsIndexJSONBytes, &listingsIndex)
//if err != nil {
//return err
//}

//var cryptoListings []interface{}
//indexv5 := []m27_ListingDatav5{}
////indexBytes0 := []byte{}

//cryptoListings = append(cryptoListings, listingsIndex...)

//// Finish early If no crypto listings
//if len(cryptoListings) == 0 {
//return writeRepoVer(repoPath, 28)
//}

//// Check each crypto listing for markup
//var markupListings []*m27_SignedListing
//for _, listingAbstract := range cryptoListings {
//listSlug := (listingAbstract.(map[string]interface{})["slug"]).(string)
//listingJSONBytes, err := ioutil.ReadFile(migration027_listingFilePath(repoPath, listSlug))
//if err != nil {
//return err
//}
//sl := new(m27_SignedListing)
//var filter m27_SignedListingDataFilter
//var temp m27_SignedListing
////err = jsonpb.UnmarshalString(string(listingJSONBytes), &temp)

//err = json.Unmarshal(listingJSONBytes, &filter)
//if err != nil {
//return err
//}

//if filter.Listing.Metadata.Version > 4 {
//b, _ := json.Marshal(listingAbstract)
////indexBytes0 = append(indexBytes0, b...)
//var n m27_ListingDatav5
//if err := json.Unmarshal(b, &n); err != nil {
//return fmt.Errorf("failed unmarshaling (%s): %s", listSlug, err.Error())
//}
//indexv5 = append(indexv5, n)
//continue
//}

//err = json.Unmarshal(listingJSONBytes, &temp)
//if err != nil {
//return err
//}

//templisting := temp.Listing
//sl.Hash = temp.Hash
//sl.Signature = temp.Signature

//sl.Listing = new(pb.Listing)
//sl.Listing.Metadata = new(pb.Listing_Metadata)
//sl.Listing.Item = new(pb.Listing_Item)

//sl.Listing.Slug = listSlug
//sl.Listing.VendorID = templisting.VendorID
//sl.Listing.Moderators = templisting.Moderators
//sl.Listing.RefundPolicy = templisting.RefundPolicy
//sl.Listing.TermsAndConditions = templisting.TermsAndConditions

//sl.Listing.Metadata.PricingCurrencyDefn = &pb.CurrencyDefinition{
//Code:         templisting.Metadata.PricingCurrency,
//Divisibility: 8,
//}

//sl.Listing.Metadata.Version = 5

//sl.Listing.Metadata.ContractType = pb.Listing_Metadata_ContractType(
//pb.Listing_Metadata_ContractType_value[templisting.Metadata.ContractType],
//)

//sl.Listing.Metadata.Format = pb.Listing_Metadata_Format(
//pb.Listing_Metadata_Format_value[templisting.Metadata.Format],
//)

//t, _ := time.Parse(time.RFC3339Nano, templisting.Metadata.Expiry)
//sl.Listing.Metadata.Expiry = &timestamp.Timestamp{
//Seconds: t.Unix(),
//Nanos:   int32(t.Nanosecond()),
//}
//sl.Listing.Metadata.AcceptedCurrencies = templisting.Metadata.AcceptedCurrencies
//sl.Listing.Metadata.EscrowTimeoutHours = templisting.Metadata.EscrowTimeoutHours
//sl.Listing.Metadata.Language = templisting.Metadata.Language
//sl.Listing.Metadata.PriceModifier = templisting.Metadata.PriceModifier

//[>
//Title                string                 `protobuf:"bytes,1,opt,name=title,proto3" json:"title,omitempty"`
//Description          string                 `protobuf:"bytes,2,opt,name=description,proto3" json:"description,omitempty"`
//ProcessingTime       string                 `protobuf:"bytes,3,opt,name=processingTime,proto3" json:"processingTime,omitempty"`
//Price                *CurrencyValue         `protobuf:"bytes,4,opt,name=price,proto3" json:"price,omitempty"`
//Nsfw                 bool                   `protobuf:"varint,5,opt,name=nsfw,proto3" json:"nsfw,omitempty"`
//Tags                 []string               `protobuf:"bytes,6,rep,name=tags,proto3" json:"tags,omitempty"`
//Images               []*Listing_Item_Image  `protobuf:"bytes,7,rep,name=images,proto3" json:"images,omitempty"`
//Categories           []string               `protobuf:"bytes,8,rep,name=categories,proto3" json:"categories,omitempty"`
//Grams                float32                `protobuf:"fixed32,9,opt,name=grams,proto3" json:"grams,omitempty"`
//Condition            string                 `protobuf:"bytes,10,opt,name=condition,proto3" json:"condition,omitempty"`
//Options              []*Listing_Item_Option `protobuf:"bytes,11,rep,name=options,proto3" json:"options,omitempty"`
//Skus                 []*Listing_Item_Sku
//*/

//sl.Listing.Item.Title = templisting.Item.Title
//sl.Listing.Item.Description = templisting.Item.Description
//sl.Listing.Item.ProcessingTime = templisting.Item.ProcessingTime
//sl.Listing.Item.BigPrice = strconv.FormatUint(templisting.Item.Price, 10)
//sl.Listing.Item.Nsfw = templisting.Item.Nsfw
//sl.Listing.Item.Tags = templisting.Item.Tags
//sl.Listing.Item.Images = templisting.Item.Images
//sl.Listing.Item.Categories = templisting.Item.Categories
//sl.Listing.Item.Grams = templisting.Item.Grams
//sl.Listing.Item.Condition = templisting.Item.Condition
//sl.Listing.Item.Options = templisting.Item.Options

//skus := []*pb.Listing_Item_Sku{}
//for _, s := range templisting.Item.Skus {
//sku := &pb.Listing_Item_Sku{
//VariantCombo: s.VariantCombo,
//ProductID:    s.ProductID,
//Quantity:     s.Quantity,
//BigSurcharge: strconv.FormatInt(s.Surcharge, 10),
//}
//skus = append(skus, sku)
//}
//sl.Listing.Item.Skus = skus

//shippingOptions := []*pb.Listing_ShippingOption{}
//for _, s := range templisting.ShippingOptions {
//regions := []pb.CountryCode{}
//for _, r := range s.Regions {
//region := pb.CountryCode(
//pb.CountryCode_value[r],
//)
//regions = append(regions, region)
//}
//sers := []*pb.Listing_ShippingOption_Service{}
//for _, s := range s.Services {
//ser := &pb.Listing_ShippingOption_Service{
//Name: s.Name,
//PriceValue: &pb.CurrencyValue{
//Currency: sl.Listing.Metadata.PricingCurrencyDefn,
//Amount:   strconv.FormatUint(s.Price, 10),
//},
//EstimatedDelivery: s.EstimatedDelivery,
//AdditionalItemPriceValue: &pb.CurrencyValue{
//Currency: sl.Listing.Metadata.PricingCurrencyDefn,
//Amount:   strconv.FormatUint(s.AdditionalItemPrice, 10),
//},
//}
//sers = append(sers, ser)
//}
//shippingOption := &pb.Listing_ShippingOption{
//Name: s.Name,
//Type: pb.Listing_ShippingOption_ShippingType(
//pb.Listing_ShippingOption_ShippingType_value[s.Type],
//),
//Regions:  regions,
//Services: sers,
//}
//shippingOptions = append(shippingOptions, shippingOption)
//}

//sl.Listing.ShippingOptions = shippingOptions

//taxes := []*pb.Listing_Tax{}

//for _, t := range templisting.Taxes {
//regions := []pb.CountryCode{}
//for _, r := range t.TaxRegions {
//region := pb.CountryCode(
//pb.CountryCode_value[r],
//)
//regions = append(regions, region)
//}
//tax := &pb.Listing_Tax{
//TaxType:     t.TaxType,
//TaxRegions:  regions,
//TaxShipping: t.TaxShipping,
//Percentage:  t.Percentage,
//}
//taxes = append(taxes, tax)
//}

//sl.Listing.Taxes = taxes

//coupons := []*pb.Listing_Coupon{}

//for _, c := range templisting.Coupons {
////discount := pb.Listing_Coupon_Discount
//coupon := &pb.Listing_Coupon{
//Title: c.Title,
////Code:  c.Code,
//}
//if c.PriceDiscount == 0 {
//disc := &pb.Listing_Coupon_PercentDiscount{
//PercentDiscount: c.PercentDiscount,
//}
//coupon.Discount = disc
//} else {
//disc := &pb.Listing_Coupon_PriceDiscountValue{
//PriceDiscountValue: &pb.CurrencyValue{
//Currency: sl.Listing.Metadata.PricingCurrencyDefn,
//Amount:   strconv.FormatUint(c.PriceDiscount, 10),
//},
//}
//coupon.Discount = disc
//}
//if c.DiscountCode == "" {
//code := &pb.Listing_Coupon_Hash{
//Hash: c.Hash,
//}
//coupon.Code = code
//} else {
//code := &pb.Listing_Coupon_DiscountCode{
//DiscountCode: c.DiscountCode,
//}
//coupon.Code = code
//}

//coupons = append(coupons, coupon)
//}

//sl.Listing.Coupons = coupons

//markupListings = append(markupListings, sl)

//}

//// Finish early If no crypto listings with new features are found
//if len(markupListings) == 0 {
//return writeRepoVer(repoPath, 28)
//}

//// Setup signing capabilities
//identityKey, err := m27_GetIdentityKey(repoPath, databasePassword, testnetEnabled)
//if err != nil {
//return err
//}

//identity, err := ipfs.IdentityFromKey(identityKey)
//if err != nil {
//return err
//}

//// IPFS node setup
//r, err := fsrepo.Open(repoPath)
//if err != nil {
//return err
//}
//cctx, cancel := context.WithCancel(context.Background())
//defer cancel()

//cfg, err := r.Config()
//if err != nil {
//return err
//}

//cfg.Identity = identity

//ncfg := &ipfscore.BuildCfg{
//Repo:   r,
//Online: false,
//ExtraOpts: map[string]bool{
//"mplex": true,
//},
//Routing: nil,
//}

//nd, err := ipfscore.NewNode(cctx, ncfg)
//if err != nil {
//return err
//}
//defer nd.Close()

//// Update each listing to have the latest version number and resave
//// Save the new hashes for each changed listing so we can update the index.
//hashes := make(map[string]string)
//amounts := make(map[string]*pb.CurrencyValue)

//privKey, err := crypto.UnmarshalPrivateKey(identityKey)
//if err != nil {
//return err
//}

//for _, sl := range markupListings {

//serializedListing, err := proto.Marshal(sl.Listing)
//if err != nil {
//return err
//}

//idSig, err := privKey.Sign(serializedListing)
//if err != nil {
//return err
//}
//sl.Signature = idSig

//m := jsonpb.Marshaler{
//EnumsAsInts:  false,
//EmitDefaults: false,
//Indent:       "    ",
//OrigName:     false,
//}
//out, err := m.MarshalToString(sl)
//if err != nil {
//return err
//}

//filename := migration027_listingFilePath(repoPath, sl.Listing.Slug)
//if err := ioutil.WriteFile(filename, []byte(out), os.ModePerm); err != nil {
//return err
//}
//h, err := ipfs.GetHashOfFile(nd, filename)
//if err != nil {
//return err
//}
//hashes[sl.Listing.Slug] = h
//amounts[sl.Listing.Slug] = sl.Listing.Item.PriceValue

//}

//// Update listing index
//indexBytes, err := ioutil.ReadFile(listingsIndexFilePath)
//if err != nil {
//return err
//}
//var index []m27_ListingData

//err = json.Unmarshal(indexBytes, &index)
//if err != nil {
//return err
//}

//for _, l := range index {
//h, ok := hashes[l.Slug]

//// Not one of the changed listings
//if !ok {
//continue
//}

//a := amounts[l.Slug]

//newListing := m27_ListingDatav5{
//Hash:               h,
//Slug:               l.Slug,
//Title:              l.Title,
//Categories:         l.Categories,
//NSFW:               l.NSFW,
//ContractType:       l.ContractType,
//Description:        l.Description,
//Thumbnail:          l.Thumbnail,
//Price:              price0{CurrencyCode: l.Price.CurrencyCode, Modifier: l.Price.Modifier, Amount: a},
//ShipsTo:            l.ShipsTo,
//FreeShipping:       l.FreeShipping,
//Language:           l.Language,
//AverageRating:      l.AverageRating,
//RatingCount:        l.RatingCount,
//ModeratorIDs:       l.ModeratorIDs,
//AcceptedCurrencies: l.AcceptedCurrencies,
//CoinType:           l.CoinType,
//}

//indexv5 = append(indexv5, newListing)
//}

//// Write it back to file
//ifile, err := os.Create(listingsIndexFilePath)
//if err != nil {
//return err
//}
//defer ifile.Close()

//j, jerr := json.MarshalIndent(indexv5, "", "    ")
//if jerr != nil {
//return jerr
//}
//_, werr := ifile.Write(j)
//if werr != nil {
//return werr
//}

//_, err = ipfs.GetHashOfFile(nd, listingsIndexFilePath)
//if err != nil {
//return err
//}

//return writeRepoVer(repoPath, 28)
//}

//func migration027_listingFilePath(datadir string, slug string) string {
//return path.Join(datadir, "root", "listings", slug+".json")
//}

//func (Migration027) Down(repoPath, databasePassword string, testnetEnabled bool) error {
//return writeRepoVer(repoPath, 27)
//}
