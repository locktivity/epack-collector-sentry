package sentry

import (
	"net/http"
	"regexp"
	"strings"
)

var linkRE = regexp.MustCompile(`<([^>]+)>;\s*rel="([^"]+)";\s*results="([^"]+)"(?:;\s*cursor="([^"]*)")?`)

type pageInfo struct {
	NextCursor string
	HasNext    bool
}

func parseLinkHeader(resp *http.Response) pageInfo {
	header := resp.Header.Get("Link")
	if header == "" {
		return pageInfo{}
	}

	for _, part := range strings.Split(header, ",") {
		matches := linkRE.FindStringSubmatch(strings.TrimSpace(part))
		if matches == nil {
			continue
		}
		rel := matches[2]
		results := matches[3]
		cursor := matches[4]

		if rel == "next" && results == "true" {
			return pageInfo{NextCursor: cursor, HasNext: true}
		}
	}

	return pageInfo{}
}
