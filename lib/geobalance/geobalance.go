package geobalance

import (
	"encoding/json"
	"log"
	"os"
	"strings"
	"sync"
)

// Map country code to continent.
var countryMappings = make(map[string]country)
var countryMu sync.Mutex

type turnInfo struct {
	url string
	key string
}

var continentsToURL = map[string]turnInfo{
	// LUX
	"AS": {url: "turn:turn-lux.airtap.dev:3478?transport=udp", key: os.Getenv("TURN_LUX_KEY")},
	"EU": {url: "turn:turn-lux.airtap.dev:3478?transport=udp", key: os.Getenv("TURN_LUX_KEY")},
	"AF": {url: "turn:turn-lux.airtap.dev:3478?transport=udp", key: os.Getenv("TURN_LUX_KEY")},

	// DFW
	"NA": {url: "turn:turn-dfw.airtap.dev:3478?transport=udp", key: os.Getenv("TURN_DFW_KEY")},
	"SA": {url: "turn:turn-dfw.airtap.dev:3478?transport=udp", key: os.Getenv("TURN_DFW_KEY")},
	"OC": {url: "turn:turn-dfw.airtap.dev:3478?transport=udp", key: os.Getenv("TURN_DFW_KEY")},
}
var continentsMu sync.Mutex

var defaultInfo = turnInfo{url: "turn:turn-lux.airtap.dev:3478?transport=udp", key: os.Getenv("TURN_LUX_KEY")}

type country struct {
	ContinentCode    string `json:"Continent_Code,omitempty"`
	ContinentName    string `json:"Continent_Name,omitempty"`
	CountryName      string `json:"Country_Name,omitempty"`
	CountryNumber    int    `json:"Country_Number,omitempty"`
	CountryCodeLong  string `json:"Three_Letter_Country_Code,omitempty"`
	CountryCodeShort string `json:"Two_Letter_Country_Code,omitempty"`
}

type countries struct {
	Countries []country `json:"Countries"`
}

func init() {
	var countries countries
	if err := json.Unmarshal([]byte(codes), &countries); err != nil {
		log.Panic(err)
	}

	for _, country := range countries.Countries {
		countryMappings[strings.ToLower(country.CountryCodeShort)] = country
	}
}

func Balance(countryCode string) (string, string) {
	countryMu.Lock()
	continentsMu.Lock()
	defer countryMu.Unlock()
	defer continentsMu.Unlock()

	// Treat Russia as North America because Ilya is in Russia and wants to test
	// the US servers. :)
	if countryCode == "RU" {
		if info, ok := continentsToURL["NA"]; ok {
			return info.url, info.key
		}
	}

	if countryInfo, ok := countryMappings[strings.ToLower(countryCode)]; ok {
		if info, ok := continentsToURL[strings.ToUpper(countryInfo.ContinentCode)]; ok {
			return info.url, info.key
		}
	}

	log.Printf("Geomapping not found for country code %v", countryCode)
	return defaultInfo.url, defaultInfo.key
}
