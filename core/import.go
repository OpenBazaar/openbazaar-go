package core

import (
	"encoding/csv"
	"errors"
	"fmt"
	"github.com/OpenBazaar/jsonpb"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/golang/protobuf/ptypes"
	"io"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"
)

func (n *OpenBazaarNode) ImportListings(r io.ReadCloser) error {
	reader := csv.NewReader(r)
	records, err := reader.ReadAll()
	if err != nil {
		return err
	}
	if len(records) < 2 {
		return errors.New("Invalid csv file")
	}
	columns := records[0]
	fields := make(map[string]int)
	for i, c := range columns {
		fields[strings.ToLower(c)] = i
	}

	type Result struct {
		listing *pb.Listing
		err     error
	}

	respCh := make(chan Result, len(records)-1)
	defer close(respCh)

	// For each row in the CSV create a new listing
	for i := 1; i < len(records); i++ {
		go func(i int) {
			listing := new(pb.Listing)
			metadata := new(pb.Listing_Metadata)
			item := new(pb.Listing_Item)
			shipping := []*pb.Listing_ShippingOption{}
			listing.Metadata = metadata
			listing.Item = item
			listing.ShippingOptions = shipping

			pos, ok := fields["contract_type"]
			if ok {
				e, ok := pb.Listing_Metadata_ContractType_value[strings.ToUpper(records[i][pos])]
				if ok {
					listing.Metadata.ContractType = pb.Listing_Metadata_ContractType(e)
				}
			}
			pos, ok = fields["format"]
			if ok {
				e, ok := pb.Listing_Metadata_Format_value[strings.ToUpper(records[i][pos])]
				if ok {
					listing.Metadata.Format = pb.Listing_Metadata_Format(e)
				}
			}
			pos, ok = fields["expiry"]
			if ok {
				t, err := time.Parse(time.RFC3339, records[i][pos])
				if err != nil {
					respCh <- Result{nil, fmt.Errorf("Error in record %d: %s", i, err.Error())}
					return
				}
				ts, err := ptypes.TimestampProto(t)
				if err != nil {
					respCh <- Result{nil, fmt.Errorf("Error in record %d: %s", i, err.Error())}
					return
				}
				listing.Metadata.Expiry = ts
			} else {
				t, err := time.Parse(time.RFC3339, "2037-12-31T05:00:00.000Z")
				if err != nil {
					respCh <- Result{nil, fmt.Errorf("Error in record %d: %s", i, err.Error())}
					return
				}
				ts, err := ptypes.TimestampProto(t)
				if err != nil {
					respCh <- Result{nil, fmt.Errorf("Error in record %d: %s", i, err.Error())}
					return
				}
				listing.Metadata.Expiry = ts
			}
			pos, ok = fields["pricing_currency"]
			if !ok {
				respCh <- Result{nil, fmt.Errorf("Error in record %d: %s", i, "pricing_currency is a mandatory field")}
				return
			}
			listing.Metadata.PricingCurrency = strings.ToUpper(records[i][pos])
			pos, ok = fields["language"]
			if ok {
				listing.Metadata.Language = records[i][pos]
			}
			pos, ok = fields["title"]
			if !ok {
				respCh <- Result{nil, fmt.Errorf("Error in record %d: %s", i, "title is a mandatory field")}
				return
			}
			listing.Item.Title = records[i][pos]

			listing.Slug, err = n.GenerateSlug(listing.Item.Title)
			if err != nil {
				respCh <- Result{nil, fmt.Errorf("Error in record %d: %s", i, err.Error())}
				return
			}

			pos, ok = fields["description"]
			if ok {
				listing.Item.Description = records[i][pos]
			}
			pos, ok = fields["processing_time"]
			if ok {
				listing.Item.ProcessingTime = records[i][pos]
			}
			pos, ok = fields["price"]
			if !ok {
				respCh <- Result{nil, fmt.Errorf("Error in record %d: %s", i, "price is a mandatory field")}
				return
			}
			if strings.ToUpper(listing.Metadata.PricingCurrency) != "BTC" {
				f, err := strconv.ParseFloat(records[i][pos], 64)
				if err != nil {
					respCh <- Result{nil, fmt.Errorf("Error in record %d: %s", i, err.Error())}
					return
				}
				listing.Item.Price = uint64(f * 100)
			} else {
				listing.Item.Price, err = strconv.ParseUint(records[i][pos], 10, 64)
				if err != nil {
					respCh <- Result{nil, fmt.Errorf("Error in record %d: %s", i, err.Error())}
					return
				}
			}
			pos, ok = fields["nsfw"]
			if ok {
				listing.Item.Nsfw, err = strconv.ParseBool(records[i][pos])
				if err != nil {
					respCh <- Result{nil, fmt.Errorf("Error in record %d: %s", i, err.Error())}
					return
				}
			}
			pos, ok = fields["tags"]
			if ok {
				listing.Item.Tags = strings.Split(records[i][pos], ",")
			}
			pos, ok = fields["image_urls"]
			if ok {
				listing.Item.Images = []*pb.Listing_Item_Image{}
				image_urls := strings.Split(records[i][pos], ",")
				var l sync.Mutex
				var wg sync.WaitGroup
				for x, img := range image_urls {
					wg.Add(1)
					go func(x int, img string) {
						defer wg.Done()
						var b64, filename string
						testUrl, err := url.Parse(img)
						if err == nil && (testUrl.Scheme == "http" || testUrl.Scheme == "https") {
							b64, filename, err = n.GetBase64Image(img)
							if err != nil {
								respCh <- Result{nil, fmt.Errorf("Error in record %d: image %d failed to download", i, x)}
								return
							}
						} else {
							filename = listing.Slug + "_" + strconv.Itoa(x)
							b64 = img
						}
						images, err := n.SetProductImages(b64, filename)
						if err != nil {
							respCh <- Result{nil, fmt.Errorf("Error in record %d: image %d invalid", i, x)}
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

				cats := strings.Split(records[i][pos], ",")
				for _, cat := range cats {
					if cat != "" {
						listing.Item.Categories = append(listing.Item.Categories, cat)
					}
				}
			}
			pos, ok = fields["condition"]
			if ok {
				listing.Item.Condition = records[i][pos]
			}
			quantityPos, quantityOK := fields["quantity"]
			skuPos, skuOK := fields["sku_number"]
			listing.Item.Skus = []*pb.Listing_Item_Sku{}
			if quantityOK || skuOK {
				sku := new(pb.Listing_Item_Sku)
				sku.ProductID = records[i][skuPos]
				if quantityOK {
					quantity, err := strconv.ParseInt(records[i][quantityPos], 10, 64)
					if err != nil {
						respCh <- Result{nil, fmt.Errorf("Error in record %d: %s", i, err.Error())}
						return
					}
					sku.Quantity = quantity
				}
				listing.Item.Skus = append(listing.Item.Skus, sku)
			}
			listing.ShippingOptions = []*pb.Listing_ShippingOption{}
			pos, ok = fields["shipping_option1_name"]
			if ok && records[i][pos] != "" {
				so := new(pb.Listing_ShippingOption)
				so.Name = records[i][pos]
				so.Type = pb.Listing_ShippingOption_FIXED_PRICE
				so.Regions = []pb.CountryCode{}
				so.Services = []*pb.Listing_ShippingOption_Service{}
				pos, ok = fields["shipping_option1_countries"]
				if ok {
					countries := strings.Split(records[i][pos], ",")
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
				if ok && records[i][pos] != "" {
					service := new(pb.Listing_ShippingOption_Service)
					service.Name = records[i][pos]
					pos, ok = fields["shipping_option1_service1_estimated_delivery"]
					if ok {
						service.EstimatedDelivery = records[i][pos]
					}
					pos, ok = fields["shipping_option1_service1_estimated_price"]
					if !ok {
						respCh <- Result{nil, fmt.Errorf("Error in record %d: %s", i, "shipping_option1_service1_estimated_price is a mandatory field")}
						return
					}
					if strings.ToUpper(listing.Metadata.PricingCurrency) != "BTC" {
						f, err := strconv.ParseFloat(records[i][pos], 64)
						if err != nil {
							respCh <- Result{nil, fmt.Errorf("Error in record %d: %s", i, err.Error())}
							return
						}
						service.Price = uint64(f * 100)
					} else {
						service.Price, err = strconv.ParseUint(records[i][pos], 10, 64)
						if err != nil {
							respCh <- Result{nil, fmt.Errorf("Error in record %d: %s", i, err.Error())}
							return
						}
					}
					so.Services = append(so.Services, service)
				}
				pos, ok = fields["shipping_option1_service2_name"]
				if ok && records[i][pos] != "" {
					service := new(pb.Listing_ShippingOption_Service)
					service.Name = records[i][pos]
					pos, ok = fields["shipping_option1_service2_estimated_delivery"]
					if ok {
						service.EstimatedDelivery = records[i][pos]
					}
					pos, ok = fields["shipping_option1_service2_estimated_price"]
					if !ok {
						respCh <- Result{nil, errors.New("shipping_option1_service2_estimated_price is a mandatory field")}
						return
					}
					if strings.ToUpper(listing.Metadata.PricingCurrency) != "BTC" {
						f, err := strconv.ParseFloat(records[i][pos], 64)
						if err != nil {
							respCh <- Result{nil, fmt.Errorf("Error in record %d: %s", i, err.Error())}
							return
						}
						service.Price = uint64(f * 100)
					} else {
						service.Price, err = strconv.ParseUint(records[i][pos], 10, 64)
						if err != nil {
							respCh <- Result{nil, fmt.Errorf("Error in record %d: %s", i, err.Error())}
							return
						}
					}
					so.Services = append(so.Services, service)
				}
				pos, ok = fields["shipping_option1_service3_name"]
				if ok && records[i][pos] != "" {
					service := new(pb.Listing_ShippingOption_Service)
					service.Name = records[i][pos]
					pos, ok = fields["shipping_option1_service3_estimated_delivery"]
					if ok {
						service.EstimatedDelivery = records[i][pos]
					}
					pos, ok = fields["shipping_option1_service3_estimated_price"]
					if !ok {
						respCh <- Result{nil, fmt.Errorf("Error in record %d: %s", i, "shipping_option1_service3_estimated_price is a mandatory field")}
						return
					}
					if strings.ToUpper(listing.Metadata.PricingCurrency) != "BTC" {
						f, err := strconv.ParseFloat(records[i][pos], 64)
						if err != nil {
							respCh <- Result{nil, fmt.Errorf("Error in record %d: %s", i, err.Error())}
							return
						}
						service.Price = uint64(f * 100)
					} else {
						service.Price, err = strconv.ParseUint(records[i][pos], 10, 64)
						if err != nil {
							respCh <- Result{nil, fmt.Errorf("Error in record %d: %s", i, err.Error())}
							return
						}
					}
					so.Services = append(so.Services, service)
				}
				listing.ShippingOptions = append(listing.ShippingOptions, so)
			}
			pos, ok = fields["shipping_option2_name"]
			if ok && records[i][pos] != "" {
				so := new(pb.Listing_ShippingOption)
				so.Name = records[i][pos]
				so.Type = pb.Listing_ShippingOption_FIXED_PRICE
				so.Regions = []pb.CountryCode{}
				so.Services = []*pb.Listing_ShippingOption_Service{}
				pos, ok = fields["shipping_option2_countries"]
				if ok {
					countries := strings.Split(records[i][pos], ",")
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
				if ok && records[i][pos] != "" {
					service := new(pb.Listing_ShippingOption_Service)
					service.Name = records[i][pos]
					pos, ok = fields["shipping_option2_service1_estimated_delivery"]
					if ok {
						service.EstimatedDelivery = records[i][pos]
					}
					pos, ok = fields["shipping_option2_service1_estimated_price"]
					if !ok {
						respCh <- Result{nil, fmt.Errorf("Error in record %d: %s", i, "shipping_option2_service1_estimated_price is a mandatory field")}
						return
					}
					if strings.ToUpper(listing.Metadata.PricingCurrency) != "BTC" {
						f, err := strconv.ParseFloat(records[i][pos], 64)
						if err != nil {
							respCh <- Result{nil, fmt.Errorf("Error in record %d: %s", i, err.Error())}
							return
						}
						service.Price = uint64(f * 100)
					} else {
						service.Price, err = strconv.ParseUint(records[i][pos], 10, 64)
						if err != nil {
							respCh <- Result{nil, fmt.Errorf("Error in record %d: %s", i, err.Error())}
							return
						}
					}
					so.Services = append(so.Services, service)
				}
				pos, ok = fields["shipping_option2_service2_name"]
				if ok && records[i][pos] != "" {
					service := new(pb.Listing_ShippingOption_Service)
					service.Name = records[i][pos]
					pos, ok = fields["shipping_option2_service2_estimated_delivery"]
					if ok {
						service.EstimatedDelivery = records[i][pos]
					}
					pos, ok = fields["shipping_option2_service2_estimated_price"]
					if !ok {
						respCh <- Result{nil, fmt.Errorf("Error in record %d: %s", i, "shipping_option2_service2_estimated_price is a mandatory field")}
						return
					}
					if strings.ToUpper(listing.Metadata.PricingCurrency) != "BTC" {
						f, err := strconv.ParseFloat(records[i][pos], 64)
						if err != nil {
							respCh <- Result{nil, fmt.Errorf("Error in record %d: %s", i, err.Error())}
							return
						}
						service.Price = uint64(f * 100)
					} else {
						service.Price, err = strconv.ParseUint(records[i][pos], 10, 64)
						if err != nil {
							respCh <- Result{nil, fmt.Errorf("Error in record %d: %s", i, err.Error())}
							return
						}
					}
					so.Services = append(so.Services, service)
				}
				pos, ok = fields["shipping_option2_service3_name"]
				if ok && records[i][pos] != "" {
					service := new(pb.Listing_ShippingOption_Service)
					service.Name = records[i][pos]
					pos, ok = fields["shipping_option2_service3_estimated_delivery"]
					if ok {
						service.EstimatedDelivery = records[i][pos]
					}
					pos, ok = fields["shipping_option2_service3_estimated_price"]
					if !ok {
						respCh <- Result{nil, fmt.Errorf("Error in record %d: %s", i, "shipping_option2_service3_estimated_price is a mandatory field")}
						return
					}
					if strings.ToUpper(listing.Metadata.PricingCurrency) != "BTC" {
						f, err := strconv.ParseFloat(records[i][pos], 64)
						if err != nil {
							respCh <- Result{nil, fmt.Errorf("Error in record %d: %s", i, err.Error())}
							return
						}
						service.Price = uint64(f * 100)
					} else {
						service.Price, err = strconv.ParseUint(records[i][pos], 10, 64)
						if err != nil {
							respCh <- Result{nil, fmt.Errorf("Error in record %d: %s", i, err.Error())}
							return
						}
					}
					so.Services = append(so.Services, service)
				}
				listing.ShippingOptions = append(listing.ShippingOptions, so)
			}
			pos, ok = fields["shipping_option3_name"]
			if ok && records[i][pos] != "" {
				so := new(pb.Listing_ShippingOption)
				so.Name = records[i][pos]
				so.Type = pb.Listing_ShippingOption_FIXED_PRICE
				so.Regions = []pb.CountryCode{}
				so.Services = []*pb.Listing_ShippingOption_Service{}
				pos, ok = fields["shipping_option3_countries"]
				if ok {
					countries := strings.Split(records[i][pos], ",")
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
				if ok && records[i][pos] != "" {
					service := new(pb.Listing_ShippingOption_Service)
					service.Name = records[i][pos]
					pos, ok = fields["shipping_option3_service1_estimated_delivery"]
					if ok {
						service.EstimatedDelivery = records[i][pos]
					}
					pos, ok = fields["shipping_option3_service1_estimated_price"]
					if !ok {
						respCh <- Result{nil, fmt.Errorf("Error in record %d: %s", i, "shipping_option3_service1_estimated_price is a mandatory field")}
						return
					}
					if strings.ToUpper(listing.Metadata.PricingCurrency) != "BTC" {
						f, err := strconv.ParseFloat(records[i][pos], 64)
						if err != nil {
							respCh <- Result{nil, fmt.Errorf("Error in record %d: %s", i, err.Error())}
							return
						}
						service.Price = uint64(f * 100)
					} else {
						service.Price, err = strconv.ParseUint(records[i][pos], 10, 64)
						if err != nil {
							respCh <- Result{nil, fmt.Errorf("Error in record %d: %s", i, err.Error())}
							return
						}
					}
					so.Services = append(so.Services, service)
				}
				pos, ok = fields["shipping_option3_service2_name"]
				if ok && records[i][pos] != "" {
					service := new(pb.Listing_ShippingOption_Service)
					service.Name = records[i][pos]
					pos, ok = fields["shipping_option3_service2_estimated_delivery"]
					if ok {
						service.EstimatedDelivery = records[i][pos]
					}
					pos, ok = fields["shipping_option3_service2_estimated_price"]
					if !ok {
						respCh <- Result{nil, fmt.Errorf("Error in record %d: %s", i, "shipping_option1_service2_estimated_price is a mandatory field")}
						return
					}
					if strings.ToUpper(listing.Metadata.PricingCurrency) != "BTC" {
						f, err := strconv.ParseFloat(records[i][pos], 64)
						if err != nil {
							respCh <- Result{nil, fmt.Errorf("Error in record %d: %s", i, err.Error())}
							return
						}
						service.Price = uint64(f * 100)
					} else {
						service.Price, err = strconv.ParseUint(records[i][pos], 10, 64)
						if err != nil {
							respCh <- Result{nil, fmt.Errorf("Error in record %d: %s", i, err.Error())}
							return
						}
					}
					so.Services = append(so.Services, service)
				}
				pos, ok = fields["shipping_option3_service3_name"]
				if ok && records[i][pos] != "" {
					service := new(pb.Listing_ShippingOption_Service)
					service.Name = records[i][pos]
					pos, ok = fields["shipping_option3_service3_estimated_delivery"]
					if ok {
						service.EstimatedDelivery = records[i][pos]
					}
					pos, ok = fields["shipping_option3_service3_estimated_price"]
					if !ok {
						respCh <- Result{nil, fmt.Errorf("Error in record %d: %s", i, "shipping_option3_service3_estimated_price is a mandatory field")}
						return
					}
					if strings.ToUpper(listing.Metadata.PricingCurrency) != "BTC" {
						f, err := strconv.ParseFloat(records[i][pos], 64)
						if err != nil {
							respCh <- Result{nil, fmt.Errorf("Error in record %d: %s", i, err.Error())}
							return
						}
						service.Price = uint64(f * 100)
					} else {
						service.Price, err = strconv.ParseUint(records[i][pos], 10, 64)
						if err != nil {
							respCh <- Result{nil, fmt.Errorf("Error in record %d: %s", i, err.Error())}
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
			respCh <- Result{listing, nil}
		}(i)
	}
	var rerr error
	var listings []*pb.Listing
	for i := 0; i < len(records)-1; i++ {
		select {
		case resp := <-respCh:
			if resp.err != nil {
				rerr = err
			} else {
				listings = append(listings, resp.listing)
			}
		}
	}
	if rerr != nil {
		return rerr
	}

	for _, listing := range listings {
		// Set inventory
		err = n.SetListingInventory(listing)
		if err != nil {
			return err
		}

		// Sign listing
		signedListing, err := n.SignListing(listing)
		if err != nil {
			return err
		}

		// Save to disk
		listingPath := path.Join(n.RepoPath, "root", "listings", signedListing.Listing.Slug+".json")
		f, err := os.Create(listingPath)
		if err != nil {
			return err
		}
		m := jsonpb.Marshaler{
			EnumsAsInts:  false,
			EmitDefaults: false,
			Indent:       "    ",
			OrigName:     false,
		}
		out, err := m.MarshalToString(signedListing)
		if err != nil {
			return err
		}

		if _, err := f.WriteString(out); err != nil {
			return err
		}

		// Update index
		err = n.UpdateListingIndex(signedListing)
		if err != nil {
			return err
		}
	}
	return nil
}
