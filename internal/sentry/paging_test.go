package sentry

import (
	"net/http"
	"testing"
)

func TestParseLinkHeader_WithNext(t *testing.T) {
	resp := &http.Response{Header: http.Header{}}
	resp.Header.Set("Link",
		`<https://sentry.io/api/0/organizations/acme/monitors/?cursor=1234:0:0>; rel="previous"; results="false"; cursor="1234:0:1", `+
			`<https://sentry.io/api/0/organizations/acme/monitors/?cursor=5678:100:0>; rel="next"; results="true"; cursor="5678:100:0"`)

	info := parseLinkHeader(resp)
	if !info.HasNext {
		t.Error("expected HasNext = true")
	}
	if info.NextCursor != "5678:100:0" {
		t.Errorf("NextCursor = %q, want %q", info.NextCursor, "5678:100:0")
	}
}

func TestParseLinkHeader_NoNext(t *testing.T) {
	resp := &http.Response{Header: http.Header{}}
	resp.Header.Set("Link",
		`<https://sentry.io/api/0/organizations/acme/monitors/?cursor=1234:0:0>; rel="previous"; results="false"; cursor="1234:0:1", `+
			`<https://sentry.io/api/0/organizations/acme/monitors/?cursor=5678:100:0>; rel="next"; results="false"; cursor="5678:100:0"`)

	info := parseLinkHeader(resp)
	if info.HasNext {
		t.Error("expected HasNext = false")
	}
}

func TestParseLinkHeader_Empty(t *testing.T) {
	resp := &http.Response{Header: http.Header{}}

	info := parseLinkHeader(resp)
	if info.HasNext {
		t.Error("expected HasNext = false for empty header")
	}
}
