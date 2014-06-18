package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

var endpoint = "http://ip-api.com/json"

type location struct {
	As          string `json:"as"`
	City        string `json:"city"`
	Country     string `json:"country"`
	CountryCode string `json:"countryCode"`
	Isp         string `json:"isp"`
	Lat         string `json:"lat"`
	Lon         string `json:"lon"`
	Org         string `json:"org"`
	Query       string `json:"query"`
	Region      string `json:"region"`
	RegionName  string `json:"regionName"`
	Status      string `json:"status"`
	Timezone    string `json:"timezone"`
	Zip         string `json:"zip"`
}

func getExitLocation(c *http.Client) (*location, error) {
	r, err := c.Get(endpoint)
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()
	if r.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Bad return code - %s", r.Status)
	}

	l := &location{}
	dec := json.NewDecoder(r.Body)
	err = dec.Decode(l)
	if err != nil {
		return nil, err
	}

	return l, nil
}
