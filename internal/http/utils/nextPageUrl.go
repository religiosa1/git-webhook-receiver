package utils

import (
	"net/http"
	"strings"
)

// BuildNextPageURL builds url for the next page, if cursor is not nil
// It uses wither publicURL from the config, or returns a relative URL otherwise.
func BuildNextPageURL(req *http.Request, publicURL string, cursor *string) *string {
	if cursor == nil {
		return nil
	}
	params := req.URL.Query()
	params.Set("cursor", *cursor)
	params.Del("offset")

	var base string
	if publicURL != "" {
		base = strings.TrimRight(publicURL, "/") + req.URL.Path
	} else {
		base = req.URL.Path
	}
	nextPageURL := base + "?" + params.Encode()
	return &nextPageURL
}
