package geobalance

import (
	"encoding/json"
	"log"
	"os"
	"strings"
)

type turnInfo struct {
	url string
	key string
}

// These maps do not need mutexes for access because they are read-only.
var (
	// Map country codes to continents.
	countryMappings = make(map[string]country)
	// Map continents to the respective TURN servers. Notice that the os.Getenv,
	// which is a syscall, is only called once during initialization.
	continentsToURL = map[string]turnInfo{
		// FRA
		"AS": {url: "turns:prod-turn-fra.airtap.dev:3478", key: os.Getenv("TURN_FRA_KEY")},
		"EU": {url: "turns:prod-turn-fra.airtap.dev:3478", key: os.Getenv("TURN_FRA_KEY")},
		"AF": {url: "turns:prod-turn-fra.airtap.dev:3478", key: os.Getenv("TURN_FRA_KEY")},

		// SFO
		"NA": {url: "turns:prod-turn-sfo.airtap.dev:3478", key: os.Getenv("TURN_SFO_KEY")},
		"SA": {url: "turns:prod-turn-sfo.airtap.dev:3478", key: os.Getenv("TURN_SFO_KEY")},
		"OC": {url: "turns:prod-turn-sfo.airtap.dev:3478", key: os.Getenv("TURN_SFO_KEY")},
	}
)

var defaultInfo = turnInfo{url: "turns:prod-turn-fra.airtap.dev:3478", key: os.Getenv("TURN_FRA_KEY")}

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

// Balance takes a country code (2 letters) and returns the URL of the TURN
// server and the key with which the password for the server should be signed.
func Balance(countryCode string) (string, string) {
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
