package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type AddressRecord struct {
	Original string
	Official string
	PrintVer string
	used     bool
}

type SmartyStreet struct {
	Scheme        string
	Host          string
	Path          string
	AuthID        string
	AuthToken     string
	MaxPerRequest int
}

var smartystreetConfigPath = filepath.Join(os.Getenv("HOME"), ".warddirectory-smartystreets.json")
var addressCachePath = filepath.Join(os.Getenv("HOME"), ".warddirectory-addresscache.json")

func (dir *Directory) CleanupAddresses() error {
	config, err := readAddressConfig(smartystreetConfigPath)

	// quit on error or config not found
	if err != nil {
		return err
	}

	// read the cache
	cache, err := readAddressCache(addressCachePath, config != nil)
	if err != nil {
		log.Printf("Error loading address cache: %v", err)
		return err
	}

	if config != nil {
		// look up any new/modified addresses
		log.Printf("Found SmartyStreets config, attempting to look up addresses")

		// gather all the addresses that need to be looked up
		missing := []string{}
		for _, f := range dir.Families {
			address := strings.Join(f.Address, "\n")
			if _, present := cache[address]; !present {
				missing = append(missing, address)
			}
		}

		// look up any new or modified addresses
		if len(missing) > 0 {
			log.Printf("Found %d new/modified addresses to look up", len(missing))

			// process new addresses into cache
			if err := config.lookupAddresses(cache, missing); err != nil {
				log.Printf("Error looking up new addresses: %v", err)
			}
		}
	}

	// now substitute in the corrected addresses
	for _, f := range dir.Families {
		address := strings.Join(f.Address, "\n")
		elt, present := cache[address]
		if !present {
			log.Printf("Address not found in cache, leaving unchanged: %q", address)
			continue
		}
		f.Address = strings.Split(elt.PrintVer, "\n")
		elt.used = true
	}

	if config != nil {
		// save the updated version of the cache
		log.Printf("Saving updated address cache to %s", addressCachePath)
		if err = writeAddressCache(addressCachePath, cache); err != nil {
			log.Printf("Error saving address cache: %s", err)
			return err
		}
	}

	return nil
}

type AddressRequest struct {
	InputID string `json:"input_id,omitempty"`
	Street  string `json:"street,omitempty"`
	Street2 string `json:"street2,omitempty"`
	City    string `json:"city,omitempty"`
	State   string `json:"state,omitempty"`
	ZipCode string `json:"zipcode,omitempty"`
}

type Components struct {
	PrimaryNumber     string `json:"primary_number"`
	StreetName        string `json:"street_name"`
	StreetSuffix      string `json:"street_suffix"`
	CityName          string `json:"city_name"`
	StateAbbreviation string `json:"state_abbreviation"`
	ZipCode           string `json:"zipcode"`
	Plus4Code         string `json:"plus4_code"`
}

type AddressResponse struct {
	InputID       string `json:"input_id"`
	DeliveryLine1 string `json:"delivery_line_1"`
	DeliveryLine2 string `json:"delivery_line_2"`
	LastLine      string `json:"last_line"`
	Components    Components
}

func (c *SmartyStreet) lookupAddresses(cache map[string]*AddressRecord, addresses []string) error {
	requestList := []*AddressRequest{}

	v := url.Values{}
	v.Set("auth-id", c.AuthID)
	v.Add("auth-token", c.AuthToken)
	url := url.URL{
		Scheme:   c.Scheme,
		Host:     c.Host,
		Path:     c.Path,
		RawQuery: v.Encode(),
	}
	log.Printf("Query URL: %s", url.String())

	// form a list of requests
	for i, elt := range addresses {
		ar := formAddressRequest(elt)
		if ar != nil {
			ar.InputID = strconv.Itoa(i)
			requestList = append(requestList, ar)
		}

		if len(requestList) >= c.MaxPerRequest || len(requestList) > 0 && i == len(addresses)-1 {
			// encode the request data
			raw, err := json.MarshalIndent(requestList, "", "    ")
			if err != nil {
				log.Fatalf("Error json encoding: %v", err)
			}

			// send the request
			log.Printf("Sending request with %d entries", len(requestList))
			resp, err := http.Post(url.String(), "application/json", bytes.NewReader(raw))
			log.Printf("Request finished")
			if err != nil {
				log.Printf("Error looking up addresses: %v", err)
				return err
			}

			// process the response
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				log.Printf("Non-okay response %d", resp.StatusCode)
				log.Printf("%#v", resp)
				return fmt.Errorf("Lookup error")
			}

			// parse result
			decoder := json.NewDecoder(resp.Body)
			var list []*AddressResponse
			if err = decoder.Decode(&list); err != nil {
				log.Printf("Error parsing JSON response: %v", err)
				return err
			}
			if len(list) == 0 {
				log.Printf("Empty response")
				log.Printf("Request:\n%s\n", string(raw))
			}

			// gather corrected addresses
			for _, a := range list {
				parts := []string{}
				if a.DeliveryLine1 != "" {
					parts = append(parts, a.DeliveryLine1)
				}
				if a.DeliveryLine2 != "" {
					parts = append(parts, a.DeliveryLine2)
				}
				streetLine := strings.Join(parts, " ")

				parts = []string{}
				if a.Components.CityName != "" {
					parts = append(parts, a.Components.CityName+",")
				}
				if a.Components.StateAbbreviation != "" {
					parts = append(parts, a.Components.StateAbbreviation)
				}
				if a.Components.ZipCode != "" {
					if a.Components.Plus4Code != "" {
						parts = append(parts, a.Components.ZipCode+"-"+a.Components.Plus4Code)
					} else {
						parts = append(parts, a.Components.ZipCode)
					}
				}
				cityLine := strings.Join(parts, " ")

				official := strings.Join([]string{streetLine, cityLine}, "\n")
				index, err := strconv.ParseInt(a.InputID, 10, 63)
				if err != nil {
					log.Printf("Error parsing InputID of %q: %v", a.InputID, err)
					return err
				}
				original := addresses[int(index)]
				cache[original] = &AddressRecord{
					Original: original,
					Official: official,
					PrintVer: official,
				}
				log.Printf("Mapping %q to %q", original, official)
			}

			requestList = []*AddressRequest{}
		}
	}

	return nil
}

func readAddressConfig(path string) (*SmartyStreet, error) {
	// load smartystreets config info; skip if not found
	data, err := ioutil.ReadFile(path)
	if err != nil && os.IsNotExist(err) {
		// config not found? continue silently
		return nil, nil
	}
	if err != nil {
		log.Printf("Error loading address config file %s: %v", path, err)
		return nil, err
	}
	config := new(SmartyStreet)
	if err = json.Unmarshal(data, &config); err != nil {
		log.Printf("Error parsing address config file %s: %v", path, err)
		return nil, err
	}

	return config, nil
}

func readAddressCache(path string, foundConfig bool) (map[string]*AddressRecord, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil && os.IsNotExist(err) {
		if foundConfig {
			log.Printf("No cache found, starting from scratch")
		}
		return make(map[string]*AddressRecord), nil
	}
	if err != nil {
		log.Printf("Error reading address cache from %s: %v", path, err)
		return nil, err
	}
	var records []*AddressRecord
	if err = json.Unmarshal(data, &records); err != nil {
		log.Printf("Error parsing address cache from %s: %v", path, err)
		return nil, err
	}

	result := make(map[string]*AddressRecord)
	for _, elt := range records {
		result[elt.Original] = elt
	}

	return result, nil
}

func writeAddressCache(path string, cache map[string]*AddressRecord) error {
	var list []*AddressRecord
	for _, elt := range cache {
		if !elt.used {
			log.Printf("Unused   : %s", elt.Original)
		}
		list = append(list, elt)
	}

	raw, err := json.MarshalIndent(list, "", "    ")
	if err != nil {
		log.Printf("Error JSON encoding cache: %v", err)
		return err
	}
	if err = ioutil.WriteFile(path, raw, 0644); err != nil {
		log.Printf("Error saving cache: %v", err)
		return err
	}
	return nil
}

var cityStateZip = regexp.MustCompile(`^(.*?), +(.*?) +(\d{5}(?: *- *\d{4})?)$`)
var cityState = regexp.MustCompile(`^(.*?), +(.*?)$`)
var zipcode = regexp.MustCompile(`\b(\d{5}(?: *- *\d{4})?)$`)

func formAddressRequest(addr string) *AddressRequest {
	lines := strings.Split(addr, "\n")
	result := &AddressRequest{}
	if len(lines) < 2 {
		log.Printf("Address needs at least 2 lines, skipping: %q", addr)
		return nil
	}

	addressLines := lines[:len(lines)-1]
	lastline := lines[len(lines)-1]
	if len(addressLines) > 0 {
		result.Street = addressLines[0]
	}
	if len(addressLines) > 1 {
		result.Street2 = addressLines[1]
	}

	// try to find City, State Zip
	groups := cityStateZip.FindStringSubmatch(lastline)
	if len(groups) > 0 {
		result.City = groups[1]
		result.State = groups[2]
		result.ZipCode = groups[3]
		return result
	}

	// try to find just Zip
	groups = zipcode.FindStringSubmatch(lastline)
	if len(groups) > 0 {
		result.ZipCode = groups[1]
		return result
	}

	// try to find City, State
	groups = cityState.FindStringSubmatch(lastline)
	if len(groups) > 0 {
		result.City = groups[1]
		result.State = groups[2]
		return result
	}

	log.Printf("Unknown address format, skipping: %q", addr)
	return nil
}
