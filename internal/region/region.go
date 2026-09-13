package region

import "strings"

var endpoints = map[string]string{
	"us": "https://api.costra.ai",
	"eu": "https://api-eu.costra.ai",
}

// Resolve returns API base URL from explicit URL or region code.
func Resolve(apiURL, apiRegion string) string {
	if u := strings.TrimRight(strings.TrimSpace(apiURL), "/"); u != "" {
		return u
	}
	r := strings.ToLower(strings.TrimSpace(apiRegion))
	if r == "" {
		r = "us"
	}
	if ep, ok := endpoints[r]; ok {
		return ep
	}
	return endpoints["us"]
}
