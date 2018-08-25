package core

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/OpenBazaar/jsonpb"
	"github.com/golang/protobuf/ptypes"

	"github.com/OpenBazaar/openbazaar-go/pb"
)

const bufferSize = 5

// ImportListings - upload/read listings
func (n *OpenBazaarNode) ImportListings(r io.ReadCloser) error {
	reader := csv.NewReader(r)
	columns, err := reader.Read()
	if err != nil {
		return err
	}
	fields := make(map[string]int)
	for i, c := range columns {
		fields[strings.ToLower(c)] = i
	}

	done := make(chan struct{})
	buf := make(chan struct{}, bufferSize)
	errChan := make(chan error, bufferSize)

	countLock := new(sync.Mutex)
	count := 0

	var ld []ListingData
	indexLock := new(sync.Mutex)
	wg := new(sync.WaitGroup)

	stringSlicePool := sync.Pool{New: func() interface{} { var s []string; return s }}

listingLoop:
	for {
		select {
		case err := <-errChan:
			return err
		case <-done:
			break listingLoop
		default:
		}
		buf <- struct{}{}
		go func() {
			defer func() {
				<-buf
			}()

			countLock.Lock()
			i := count
			_, ok := stringSlicePool.Get().([]string)
			if !ok {
				errChan <- fmt.Errorf("Error in record %d: %s", i, "[]string pool contains non []string")
				return
			}
			record, err := reader.Read()
			count++
			countLock.Unlock()
			if err == io.EOF {
				done <- struct{}{}
				return
			}
			if err != nil {
				errChan <- fmt.Errorf("Error in record %d: %s", i, err.Error())
				return
			}

			wg.Add(1)

			listing := new(pb.Listing)
			metadata := new(pb.Listing_Metadata)
			item := new(pb.Listing_Item)
			shipping := []*pb.Listing_ShippingOption{}
			listing.Metadata = metadata
			listing.Item = item
			listing.ShippingOptions = shipping

			pos, ok := fields["contract_type"]
			if ok {
				e, ok := pb.Listing_Metadata_ContractType_value[strings.ToUpper(record[pos])]
				if ok {
					listing.Metadata.ContractType = pb.Listing_Metadata_ContractType(e)
				}
			}
			pos, ok = fields["format"]
			if ok {
				e, ok := pb.Listing_Metadata_Format_value[strings.ToUpper(record[pos])]
				if ok {
					listing.Metadata.Format = pb.Listing_Metadata_Format(e)
				}
			}
			pos, ok = fields["expiry"]
			if ok {
				t, err := time.Parse(time.RFC3339, record[pos])
				if err != nil {
					errChan <- fmt.Errorf("Error in record %d: %s", i, err.Error())
					return
				}
				ts, err := ptypes.TimestampProto(t)
				if err != nil {
					errChan <- fmt.Errorf("Error in record %d: %s", i, err.Error())
					return
				}
				listing.Metadata.Expiry = ts
			} else {
				t, err := time.Parse(time.RFC3339, "2037-12-31T05:00:00.000Z")
				if err != nil {
					errChan <- fmt.Errorf("Error in record %d: %s", i, err.Error())
					return
				}
				ts, err := ptypes.TimestampProto(t)
				if err != nil {
					errChan <- fmt.Errorf("Error in record %d: %s", i, err.Error())
					return
				}
				listing.Metadata.Expiry = ts
			}
			pos, ok = fields["pricing_currency"]
			if !ok {
				errChan <- fmt.Errorf("Error in record %d: %s", i, "pricing_currency is a mandatory field")
				return
			}
			listing.Metadata.PricingCurrency = strings.ToUpper(record[pos])
			pos, ok = fields["language"]
			if ok {
				listing.Metadata.Language = record[pos]
			}
			pos, ok = fields["title"]
			if !ok {
				errChan <- fmt.Errorf("Error in record %d: %s", i, "title is a mandatory field")
				return
			}
			listing.Item.Title = record[pos]

			listing.Slug, err = n.GenerateSlug(listing.Item.Title)
			if err != nil {
				errChan <- fmt.Errorf("Error in record %d: %s", i, err.Error())
				return
			}

			pos, ok = fields["description"]
			if ok {
				listing.Item.Description = record[pos]
			}
			pos, ok = fields["processing_time"]
			if ok {
				listing.Item.ProcessingTime = record[pos]
			}
			pos, ok = fields["price"]
			if !ok {
				errChan <- fmt.Errorf("Error in record %d: %s", i, "price is a mandatory field")
				return
			}
			if NormalizeCurrencyCode(listing.Metadata.PricingCurrency) != "BTC" {
				f, err := strconv.ParseFloat(record[pos], 64)
				if err != nil {
					errChan <- fmt.Errorf("Error in record %d: %s", i, err.Error())
					return
				}
				listing.Item.Price = uint64(f * 100)
			} else {
				listing.Item.Price, err = strconv.ParseUint(record[pos], 10, 64)
				if err != nil {
					errChan <- fmt.Errorf("Error in record %d: %s", i, err.Error())
					return
				}
			}
			pos, ok = fields["nsfw"]
			if ok {
				listing.Item.Nsfw, err = strconv.ParseBool(record[pos])
				if err != nil {
					errChan <- fmt.Errorf("Error in record %d: %s", i, err.Error())
					return
				}
			}
			pos, ok = fields["tags"]
			if ok {
				listing.Item.Tags = strings.Split(record[pos], ",")
			}
			pos, ok = fields["image_urls"]
			if ok {
				listing.Item.Images = []*pb.Listing_Item_Image{}
				imageUrls := strings.Split(record[pos], ",")
				var l sync.Mutex
				var wg sync.WaitGroup
				var x int
				var img string
				for x, img = range imageUrls {
					wg.Add(1)
					go func(x int, img string) {
						defer wg.Done()
						var b64 string
						var filename string
						testURL, err := url.Parse(img)
						if err == nil && (testURL.Scheme == "http" || testURL.Scheme == "https") {
							b64, filename, err = n.GetBase64Image(img)
							if err != nil {
								errChan <- fmt.Errorf("Error in record %d: image %d failed to download", i, x)
								return
							}
						} else {
							filename = listing.Slug + "_" + strconv.Itoa(x)
							b64 = img
						}
						images, err := n.SetProductImages(b64, filename)
						if err != nil {
							errChan <- fmt.Errorf("Error in record %d: image %d invalid", i, x)
							return
						}
						imgpb := &pb.Listing_Item_Image{
							Filename: filename,
							Tiny:     images.Tiny,
							Small:    images.Small,
							Medium:   images.Medium,
							Large:    images.Large,
							Original: images.Original,
						}
						l.Lock()
						listing.Item.Images = append(listing.Item.Images, imgpb)
						l.Unlock()
					}(x, img)
				}
				wg.Wait()
			}
			pos, ok = fields["categories"]
			if ok {

				cats := strings.Split(record[pos], ",")
				for _, cat := range cats {
					if cat != "" {
						listing.Item.Categories = append(listing.Item.Categories, cat)
					}
				}
			}
			pos, ok = fields["condition"]
			if ok {
				listing.Item.Condition = record[pos]
			}
			quantityPos, quantityOK := fields["quantity"]
			skuPos, skuOK := fields["sku_number"]
			listing.Item.Skus = []*pb.Listing_Item_Sku{}
			if quantityOK || skuOK {
				sku := new(pb.Listing_Item_Sku)
				sku.ProductID = record[skuPos]
				if quantityOK {
					quantity, err := strconv.ParseInt(record[quantityPos], 10, 64)
					if err != nil {
						errChan <- fmt.Errorf("Error in record %d: %s", i, err.Error())
						return
					}
					sku.Quantity = quantity
				}
				listing.Item.Skus = append(listing.Item.Skus, sku)
			}
			listing.ShippingOptions = []*pb.Listing_ShippingOption{}
			pos, ok = fields["shipping_option1_name"]
			if ok && record[pos] != "" {
				so := new(pb.Listing_ShippingOption)
				so.Name = record[pos]
				so.Type = pb.Listing_ShippingOption_FIXED_PRICE
				so.Regions = []pb.CountryCode{}
				so.Services = []*pb.Listing_ShippingOption_Service{}
				pos, ok = fields["shipping_option1_countries"]
				if ok {
					countries := strings.Split(record[pos], ",")
					for _, c := range countries {
						e, ok := pb.CountryCode_value[strings.ToUpper(c)]
						if ok {
							so.Regions = append(so.Regions, pb.CountryCode(e))
						}
					}
				} else {
					so.Regions = append(so.Regions, pb.CountryCode_ALL)
				}
				pos, ok = fields["shipping_option1_service1_name"]
				if ok && record[pos] != "" {
					service := new(pb.Listing_ShippingOption_Service)
					service.Name = record[pos]
					pos, ok = fields["shipping_option1_service1_estimated_delivery"]
					if ok {
						service.EstimatedDelivery = record[pos]
					}
					pos, ok = fields["shipping_option1_service1_estimated_price"]
					if !ok {
						errChan <- fmt.Errorf("Error in record %d: %s", i, "shipping_option1_service1_estimated_price is a mandatory field")
						return
					}
					if NormalizeCurrencyCode(listing.Metadata.PricingCurrency) != "BTC" {
						f, err := strconv.ParseFloat(record[pos], 64)
						if err != nil {
							errChan <- fmt.Errorf("Error in record %d: %s", i, err.Error())
							return
						}
						service.Price = uint64(f * 100)
					} else {
						service.Price, err = strconv.ParseUint(record[pos], 10, 64)
						if err != nil {
							errChan <- fmt.Errorf("Error in record %d: %s", i, err.Error())
							return
						}
					}
					so.Services = append(so.Services, service)
				}
				pos, ok = fields["shipping_option1_service2_name"]
				if ok && record[pos] != "" {
					service := new(pb.Listing_ShippingOption_Service)
					service.Name = record[pos]
					pos, ok = fields["shipping_option1_service2_estimated_delivery"]
					if ok {
						service.EstimatedDelivery = record[pos]
					}
					pos, ok = fields["shipping_option1_service2_estimated_price"]
					if !ok {
						errChan <- errors.New("shipping_option1_service2_estimated_price is a mandatory field")
						return
					}
					if NormalizeCurrencyCode(listing.Metadata.PricingCurrency) != "BTC" {
						f, err := strconv.ParseFloat(record[pos], 64)
						if err != nil {
							errChan <- fmt.Errorf("Error in record %d: %s", i, err.Error())
							return
						}
						service.Price = uint64(f * 100)
					} else {
						service.Price, err = strconv.ParseUint(record[pos], 10, 64)
						if err != nil {
							errChan <- fmt.Errorf("Error in record %d: %s", i, err.Error())
							return
						}
					}
					so.Services = append(so.Services, service)
				}
				pos, ok = fields["shipping_option1_service3_name"]
				if ok && record[pos] != "" {
					service := new(pb.Listing_ShippingOption_Service)
					service.Name = record[pos]
					pos, ok = fields["shipping_option1_service3_estimated_delivery"]
					if ok {
						service.EstimatedDelivery = record[pos]
					}
					pos, ok = fields["shipping_option1_service3_estimated_price"]
					if !ok {
						errChan <- fmt.Errorf("Error in record %d: %s", i, "shipping_option1_service3_estimated_price is a mandatory field")
						return
					}
					if NormalizeCurrencyCode(listing.Metadata.PricingCurrency) != "BTC" {
						f, err := strconv.ParseFloat(record[pos], 64)
						if err != nil {
							errChan <- fmt.Errorf("Error in record %d: %s", i, err.Error())
							return
						}
						service.Price = uint64(f * 100)
					} else {
						service.Price, err = strconv.ParseUint(record[pos], 10, 64)
						if err != nil {
							errChan <- fmt.Errorf("Error in record %d: %s", i, err.Error())
							return
						}
					}
					so.Services = append(so.Services, service)
				}
				listing.ShippingOptions = append(listing.ShippingOptions, so)
			}
			pos, ok = fields["shipping_option2_name"]
			if ok && record[pos] != "" {
				so := new(pb.Listing_ShippingOption)
				so.Name = record[pos]
				so.Type = pb.Listing_ShippingOption_FIXED_PRICE
				so.Regions = []pb.CountryCode{}
				so.Services = []*pb.Listing_ShippingOption_Service{}
				pos, ok = fields["shipping_option2_countries"]
				if ok {
					countries := strings.Split(record[pos], ",")
					for _, c := range countries {
						e, ok := pb.CountryCode_value[strings.ToUpper(c)]
						if ok {
							so.Regions = append(so.Regions, pb.CountryCode(e))
						}
					}
				} else {
					so.Regions = append(so.Regions, pb.CountryCode_ALL)
				}
				pos, ok = fields["shipping_option2_service1_name"]
				if ok && record[pos] != "" {
					service := new(pb.Listing_ShippingOption_Service)
					service.Name = record[pos]
					pos, ok = fields["shipping_option2_service1_estimated_delivery"]
					if ok {
						service.EstimatedDelivery = record[pos]
					}
					pos, ok = fields["shipping_option2_service1_estimated_price"]
					if !ok {
						errChan <- fmt.Errorf("Error in record %d: %s", i, "shipping_option2_service1_estimated_price is a mandatory field")
						return
					}
					if NormalizeCurrencyCode(listing.Metadata.PricingCurrency) != "BTC" {
						f, err := strconv.ParseFloat(record[pos], 64)
						if err != nil {
							errChan <- fmt.Errorf("Error in record %d: %s", i, err.Error())
							return
						}
						service.Price = uint64(f * 100)
					} else {
						service.Price, err = strconv.ParseUint(record[pos], 10, 64)
						if err != nil {
							errChan <- fmt.Errorf("Error in record %d: %s", i, err.Error())
							return
						}
					}
					so.Services = append(so.Services, service)
				}
				pos, ok = fields["shipping_option2_service2_name"]
				if ok && record[pos] != "" {
					service := new(pb.Listing_ShippingOption_Service)
					service.Name = record[pos]
					pos, ok = fields["shipping_option2_service2_estimated_delivery"]
					if ok {
						service.EstimatedDelivery = record[pos]
					}
					pos, ok = fields["shipping_option2_service2_estimated_price"]
					if !ok {
						errChan <- fmt.Errorf("Error in record %d: %s", i, "shipping_option2_service2_estimated_price is a mandatory field")
						return
					}
					if NormalizeCurrencyCode(listing.Metadata.PricingCurrency) != "BTC" {
						f, err := strconv.ParseFloat(record[pos], 64)
						if err != nil {
							errChan <- fmt.Errorf("Error in record %d: %s", i, err.Error())
							return
						}
						service.Price = uint64(f * 100)
					} else {
						service.Price, err = strconv.ParseUint(record[pos], 10, 64)
						if err != nil {
							errChan <- fmt.Errorf("Error in record %d: %s", i, err.Error())
							return
						}
					}
					so.Services = append(so.Services, service)
				}
				pos, ok = fields["shipping_option2_service3_name"]
				if ok && record[pos] != "" {
					service := new(pb.Listing_ShippingOption_Service)
					service.Name = record[pos]
					pos, ok = fields["shipping_option2_service3_estimated_delivery"]
					if ok {
						service.EstimatedDelivery = record[pos]
					}
					pos, ok = fields["shipping_option2_service3_estimated_price"]
					if !ok {
						errChan <- fmt.Errorf("Error in record %d: %s", i, "shipping_option2_service3_estimated_price is a mandatory field")
						return
					}
					if NormalizeCurrencyCode(listing.Metadata.PricingCurrency) != "BTC" {
						f, err := strconv.ParseFloat(record[pos], 64)
						if err != nil {
							errChan <- fmt.Errorf("Error in record %d: %s", i, err.Error())
							return
						}
						service.Price = uint64(f * 100)
					} else {
						service.Price, err = strconv.ParseUint(record[pos], 10, 64)
						if err != nil {
							errChan <- fmt.Errorf("Error in record %d: %s", i, err.Error())
							return
						}
					}
					so.Services = append(so.Services, service)
				}
				listing.ShippingOptions = append(listing.ShippingOptions, so)
			}
			pos, ok = fields["shipping_option3_name"]
			if ok && record[pos] != "" {
				so := new(pb.Listing_ShippingOption)
				so.Name = record[pos]
				so.Type = pb.Listing_ShippingOption_FIXED_PRICE
				so.Regions = []pb.CountryCode{}
				so.Services = []*pb.Listing_ShippingOption_Service{}
				pos, ok = fields["shipping_option3_countries"]
				if ok {
					countries := strings.Split(record[pos], ",")
					for _, c := range countries {
						e, ok := pb.CountryCode_value[strings.ToUpper(c)]
						if ok {
							so.Regions = append(so.Regions, pb.CountryCode(e))
						}
					}
				} else {
					so.Regions = append(so.Regions, pb.CountryCode_ALL)
				}
				pos, ok = fields["shipping_option3_service1_name"]
				if ok && record[pos] != "" {
					service := new(pb.Listing_ShippingOption_Service)
					service.Name = record[pos]
					pos, ok = fields["shipping_option3_service1_estimated_delivery"]
					if ok {
						service.EstimatedDelivery = record[pos]
					}
					pos, ok = fields["shipping_option3_service1_estimated_price"]
					if !ok {
						errChan <- fmt.Errorf("Error in record %d: %s", i, "shipping_option3_service1_estimated_price is a mandatory field")
						return
					}
					if NormalizeCurrencyCode(listing.Metadata.PricingCurrency) != "BTC" {
						f, err := strconv.ParseFloat(record[pos], 64)
						if err != nil {
							errChan <- fmt.Errorf("Error in record %d: %s", i, err.Error())
							return
						}
						service.Price = uint64(f * 100)
					} else {
						service.Price, err = strconv.ParseUint(record[pos], 10, 64)
						if err != nil {
							errChan <- fmt.Errorf("Error in record %d: %s", i, err.Error())
							return
						}
					}
					so.Services = append(so.Services, service)
				}
				pos, ok = fields["shipping_option3_service2_name"]
				if ok && record[pos] != "" {
					service := new(pb.Listing_ShippingOption_Service)
					service.Name = record[pos]
					pos, ok = fields["shipping_option3_service2_estimated_delivery"]
					if ok {
						service.EstimatedDelivery = record[pos]
					}
					pos, ok = fields["shipping_option3_service2_estimated_price"]
					if !ok {
						errChan <- fmt.Errorf("Error in record %d: %s", i, "shipping_option1_service2_estimated_price is a mandatory field")
						return
					}
					if NormalizeCurrencyCode(listing.Metadata.PricingCurrency) != "BTC" {
						f, err := strconv.ParseFloat(record[pos], 64)
						if err != nil {
							errChan <- fmt.Errorf("Error in record %d: %s", i, err.Error())
							return
						}
						service.Price = uint64(f * 100)
					} else {
						service.Price, err = strconv.ParseUint(record[pos], 10, 64)
						if err != nil {
							errChan <- fmt.Errorf("Error in record %d: %s", i, err.Error())
							return
						}
					}
					so.Services = append(so.Services, service)
				}
				pos, ok = fields["shipping_option3_service3_name"]
				if ok && record[pos] != "" {
					service := new(pb.Listing_ShippingOption_Service)
					service.Name = record[pos]
					pos, ok = fields["shipping_option3_service3_estimated_delivery"]
					if ok {
						service.EstimatedDelivery = record[pos]
					}
					pos, ok = fields["shipping_option3_service3_estimated_price"]
					if !ok {
						errChan <- fmt.Errorf("Error in record %d: %s", i, "shipping_option3_service3_estimated_price is a mandatory field")
						return
					}
					if NormalizeCurrencyCode(listing.Metadata.PricingCurrency) != "BTC" {
						f, err := strconv.ParseFloat(record[pos], 64)
						if err != nil {
							errChan <- fmt.Errorf("Error in record %d: %s", i, err.Error())
							return
						}
						service.Price = uint64(f * 100)
					} else {
						service.Price, err = strconv.ParseUint(record[pos], 10, 64)
						if err != nil {
							errChan <- fmt.Errorf("Error in record %d: %s", i, err.Error())
							return
						}
					}
					so.Services = append(so.Services, service)
				}
				listing.ShippingOptions = append(listing.ShippingOptions, so)
			}
			// Set moderators
			if len(listing.Moderators) == 0 {
				sd, err := n.Datastore.Settings().Get()
				if err == nil && sd.StoreModerators != nil {
					listing.Moderators = *sd.StoreModerators
				}
			}

			// Set inventory
			err = n.SetListingInventory(listing)
			if err != nil {
				errChan <- fmt.Errorf("Error in record %d: %s", i, err.Error())
				return
			}

			// Sign listing
			signedListing, err := n.SignListing(listing)
			if err != nil {
				errChan <- fmt.Errorf("Error in record %d: %s", i, err.Error())
				return
			}

			// Save to disk
			listingPath := path.Join(n.RepoPath, "root", "listings", signedListing.Listing.Slug+".json")
			f, err := os.Create(listingPath)
			if err != nil {
				errChan <- fmt.Errorf("Error in record %d: %s", i, err.Error())
				return
			}
			defer f.Close()
			m := jsonpb.Marshaler{
				EnumsAsInts:  false,
				EmitDefaults: false,
				Indent:       "    ",
				OrigName:     false,
			}
			out, err := m.MarshalToString(signedListing)
			if err != nil {
				errChan <- fmt.Errorf("Error in record %d: %s", i, err.Error())
				return
			}

			if _, err := f.WriteString(out); err != nil {
				errChan <- fmt.Errorf("Error in record %d: %s", i, err.Error())
				return
			}

			// Add listing data
			data, err := n.extractListingData(signedListing)
			if err != nil {
				errChan <- fmt.Errorf("Error extractinng listings: %s", err.Error())
				return
			}

			indexLock.Lock()
			ld = append(ld, data)
			indexLock.Unlock()
			wg.Done()
		}()
	}
	wg.Wait()
	select {
	case err := <-errChan:
		return err
	default:
	}
	index, err := n.getListingIndex()
	if err != nil {
		return err
	}
	index = append(index, ld...)

	// Write it back to file
	indexPath := path.Join(n.RepoPath, "root", "listings.json")
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
