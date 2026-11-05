package sentry

import (
	"net/http"
	"net/url"

	"github.com/locktivity/epack/componentsdk"
)

// nextPage returns the URL of the page after resp. Every page of a Sentry
// list has a next link. On the last page, that link has results="false",
// which componentsdk.NextLink skips.
func nextPage(resp *http.Response) (*url.URL, bool) {
	return componentsdk.NextLink(resp)
}
