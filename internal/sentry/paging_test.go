package sentry

import (
	"net/http"
	"testing"
)

const pagedURL = "https://sentry.io/api/0/organizations/acme/monitors/"

func linkResponse(link string) *http.Response {
	resp := &http.Response{Header: http.Header{}}
	if link != "" {
		resp.Header.Set("Link", link)
	}
	return resp
}

func TestNextPageFollowsTheNextLink(t *testing.T) {
	resp := linkResponse(`<` + pagedURL + `?cursor=1234:0:1>; rel="previous"; results="false"; cursor="1234:0:1", ` +
		`<` + pagedURL + `?cursor=5678:100:0>; rel="next"; results="true"; cursor="5678:100:0"`)

	next, ok := nextPage(resp)
	if !ok || next.String() != pagedURL+"?cursor=5678:100:0" {
		t.Errorf("nextPage = %v, %v; want %s?cursor=5678:100:0", next, ok, pagedURL)
	}
}

func TestNextPageStopsWhenTheNextLinkHasNoResults(t *testing.T) {
	resp := linkResponse(`<` + pagedURL + `?cursor=1234:0:1>; rel="previous"; results="true"; cursor="1234:0:1", ` +
		`<` + pagedURL + `?cursor=5678:100:0>; rel="next"; results="false"; cursor="5678:100:0"`)

	if next, ok := nextPage(resp); ok {
		t.Errorf("nextPage = %v, want no next page", next)
	}
}

func TestNextPageFollowsANextLinkWithoutResults(t *testing.T) {
	resp := linkResponse(`<` + pagedURL + `?cursor=5678:100:0>; rel="next"`)

	if next, ok := nextPage(resp); !ok || next.String() != pagedURL+"?cursor=5678:100:0" {
		t.Errorf("nextPage = %v, %v; want %s?cursor=5678:100:0", next, ok, pagedURL)
	}
}

func TestNextPageWithoutALinkHeader(t *testing.T) {
	if next, ok := nextPage(linkResponse("")); ok {
		t.Errorf("nextPage = %v, want no next page", next)
	}
}
