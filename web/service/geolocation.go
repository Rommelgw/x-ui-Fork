// Package service provides geolocation services for IP address lookup.
package service

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v2/logger"
	"github.com/mhsanaei/3x-ui/v2/util/common"
)

// GeoLocation represents geographical location information for an IP address.
type GeoLocation struct {
	Country   string  `json:"country"`
	City      string  `json:"city"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Location  string  `json:"location"` // General location description
}

// GeolocationService provides IP geolocation lookup functionality.
type GeolocationService struct {
	client *http.Client
}

// NewGeolocationService creates a new geolocation service instance.
func NewGeolocationService() *GeolocationService {
	return &GeolocationService{
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// GetLocationByIP determines the geographical location of an IP address.
// It tries multiple free geolocation APIs as fallbacks.
func (s *GeolocationService) GetLocationByIP(ip string) (*GeoLocation, error) {
	// Validate IP address
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		// Try to resolve hostname to IP
		ips, err := net.LookupIP(ip)
		if err != nil || len(ips) == 0 {
			return nil, common.NewErrorf("invalid IP address or hostname: %s", ip)
		}
		ip = ips[0].String()
		parsedIP = net.ParseIP(ip)
		if parsedIP == nil {
			return nil, common.NewErrorf("failed to resolve IP address: %s", ip)
		}
	}

	// Skip private/local IPs
	if parsedIP.IsLoopback() || parsedIP.IsPrivate() || parsedIP.IsLinkLocalUnicast() {
		return nil, common.NewError("cannot determine location for private/local IP address")
	}

	// Try ip-api.com first (free, no API key required, 45 req/min)
	location, err := s.getLocationFromIPAPI(ip)
	if err == nil && location != nil {
		logger.Debugf("GeolocationService: successfully got location from ip-api.com for %s", ip)
		return location, nil
	}
	logger.Debugf("GeolocationService: ip-api.com failed for %s: %v", ip, err)

	// Fallback to ipapi.co
	location, err = s.getLocationFromIPAPICo(ip)
	if err == nil && location != nil {
		logger.Debugf("GeolocationService: successfully got location from ipapi.co for %s", ip)
		return location, nil
	}
	logger.Debugf("GeolocationService: ipapi.co failed for %s: %v", ip, err)

	// Fallback to geojs.io
	location, err = s.getLocationFromGeoJS(ip)
	if err == nil && location != nil {
		logger.Debugf("GeolocationService: successfully got location from geojs.io for %s", ip)
		return location, nil
	}
	logger.Debugf("GeolocationService: geojs.io failed for %s: %v", ip, err)

	return nil, common.NewErrorf("failed to determine location for IP %s: all geolocation services failed", ip)
}

// getLocationFromIPAPI uses ip-api.com service (free, 45 req/min)
func (s *GeolocationService) getLocationFromIPAPI(ip string) (*GeoLocation, error) {
	url := fmt.Sprintf("http://ip-api.com/json/%s?fields=status,message,country,city,lat,lon", ip)
	
	resp, err := s.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, common.NewErrorf("ip-api.com returned status %d", resp.StatusCode)
	}

	var result struct {
		Status  string  `json:"status"`
		Message string  `json:"message"`
		Country string  `json:"country"`
		City    string  `json:"city"`
		Lat     float64 `json:"lat"`
		Lon     float64 `json:"lon"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if result.Status != "success" {
		return nil, common.NewErrorf("ip-api.com error: %s", result.Message)
	}

	if result.Lat == 0 && result.Lon == 0 {
		return nil, common.NewError("ip-api.com returned invalid coordinates")
	}

	location := &GeoLocation{
		Country:   result.Country,
		City:      result.City,
		Latitude:  result.Lat,
		Longitude: result.Lon,
	}
	location.Location = s.buildLocationString(location.Country, location.City)

	return location, nil
}

// getLocationFromIPAPICo uses ipapi.co service (free, 1000 req/day)
func (s *GeolocationService) getLocationFromIPAPICo(ip string) (*GeoLocation, error) {
	url := fmt.Sprintf("https://ipapi.co/%s/json/", ip)
	
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "3x-ui-fork/1.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, common.NewErrorf("ipapi.co returned status %d", resp.StatusCode)
	}

	var result struct {
		Error     bool    `json:"error"`
		Reason    string  `json:"reason"`
		Country   string  `json:"country_name"`
		City      string  `json:"city"`
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if result.Error {
		return nil, common.NewErrorf("ipapi.co error: %s", result.Reason)
	}

	if result.Latitude == 0 && result.Longitude == 0 {
		return nil, common.NewError("ipapi.co returned invalid coordinates")
	}

	location := &GeoLocation{
		Country:   result.Country,
		City:      result.City,
		Latitude:  result.Latitude,
		Longitude: result.Longitude,
	}
	location.Location = s.buildLocationString(location.Country, location.City)

	return location, nil
}

// getLocationFromGeoJS uses geojs.io service (free, no limits)
func (s *GeolocationService) getLocationFromGeoJS(ip string) (*GeoLocation, error) {
	url := fmt.Sprintf("https://get.geojs.io/v1/ip/geo/%s.json", ip)
	
	resp, err := s.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, common.NewErrorf("geojs.io returned status %d", resp.StatusCode)
	}

	var result struct {
		Country   string  `json:"country"`
		City      string  `json:"city"`
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if result.Latitude == 0 && result.Longitude == 0 {
		return nil, common.NewError("geojs.io returned invalid coordinates")
	}

	location := &GeoLocation{
		Country:   result.Country,
		City:      result.City,
		Latitude:  result.Latitude,
		Longitude: result.Longitude,
	}
	location.Location = s.buildLocationString(location.Country, location.City)

	return location, nil
}

// buildLocationString creates a location description from country and city.
func (s *GeolocationService) buildLocationString(country, city string) string {
	parts := []string{}
	if city != "" {
		parts = append(parts, city)
	}
	if country != "" {
		parts = append(parts, country)
	}
	if len(parts) > 0 {
		return strings.Join(parts, ", ")
	}
	return ""
}

