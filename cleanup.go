package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
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
	if err != nil || config == nil {
		return err
	}
	log.Printf("Found SmartyStreets config, attempting to clean up addresses")

	// read the cache
	cache, err := readAddressCache(addressCachePath)
	if err != nil {
		log.Printf("Error loading address cache: %v", err)
		return err
	}

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
		if err := lookupAddresses(cache, missing); err != nil {
			log.Printf("Error looking up new addresses: %v", err)
			return err
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

	// save the updated version of the cache
	if err = writeAddressCache(addressCachePath, cache); err != nil {
		log.Printf("Error saving address cache: %s", err)
		return err
	}

	return nil
}

func lookupAddresses(cache map[string]*AddressRecord, addresses []string) error {
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

func readAddressCache(path string) (map[string]*AddressRecord, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil && os.IsNotExist(err) {
		log.Printf("No cache found, starting from scratch")
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
