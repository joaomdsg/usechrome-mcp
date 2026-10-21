package dsl

import (
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

// Site is a local web server for the fixture pages in testdata.
type Site struct {
	srv *httptest.Server
}

// NewSite serves the fixtures for the test. Besides the files in testdata it
// has /redirect, a 302 to /second.html; any other path is a 404 with a body.
func NewSite(t testing.TB) *Site {
	t.Helper()
	// go test runs in the package directory, so this is acceptance/testdata.
	if _, err := os.Stat("testdata"); err != nil {
		t.Fatalf("fixtures: %v", err)
	}
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir("testdata")))
	mux.Handle("/redirect", http.RedirectHandler("/second.html", http.StatusFound))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return &Site{srv: srv}
}

// URL returns the address of path on the site.
func (s *Site) URL(path string) string {
	return s.srv.URL + path
}

// UnreachableURL returns an address that refuses connections.
func UnreachableURL(t testing.TB) string {
	t.Helper()
	// A port that was free a moment ago refuses connections, and unlike low
	// ports it is not on Chrome's unsafe-port list.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	return "http://" + addr + "/"
}
