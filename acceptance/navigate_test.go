//go:build acceptance

package acceptance_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/joaomdsg/usechrome-mcp/acceptance/dsl"
)

func TestOpenFollowsRedirectToFinalPage(t *testing.T) {
	t.Parallel()
	site := dsl.NewSite(t)
	got := dsl.NewAgent(t).Open(site.URL("/redirect"))
	want := dsl.Page{URL: site.URL("/second.html"), Status: http.StatusOK, Title: "Second Page"}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestOpenReportsNotFoundStatus(t *testing.T) {
	t.Parallel()
	site := dsl.NewSite(t)
	if got := dsl.NewAgent(t).Open(site.URL("/missing")); got.Status != http.StatusNotFound {
		t.Errorf("got %+v, want status 404", got)
	}
}

func TestBackAndForwardRevisitPages(t *testing.T) {
	t.Parallel()
	site := dsl.NewSite(t)
	agent := dsl.NewAgent(t)
	agent.Open(site.URL("/"))
	agent.Open(site.URL("/second.html"))

	if got := agent.GoBack(); got.URL != site.URL("/") || got.Title != "Fixture Home" {
		t.Errorf("after back: %+v, want Fixture Home", got)
	}
	if got := agent.GoForward(); got.URL != site.URL("/second.html") || got.Title != "Second Page" {
		t.Errorf("after forward: %+v, want Second Page", got)
	}
}

func TestReloadStaysOnPage(t *testing.T) {
	t.Parallel()
	site := dsl.NewSite(t)
	agent := dsl.NewAgent(t)
	agent.Open(site.URL("/second.html"))
	got := agent.Reload()
	want := dsl.Page{URL: site.URL("/second.html"), Status: http.StatusOK, Title: "Second Page"}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestBackWithoutHistoryFails(t *testing.T) {
	t.Parallel()
	err := dsl.NewAgent(t).TryGoBack()
	if err == nil || !strings.Contains(err.Error(), "no page to go back to") {
		t.Errorf("err = %v, want no page to go back to", err)
	}
}

func TestOpenUnreachableHostFails(t *testing.T) {
	t.Parallel()
	err := dsl.NewAgent(t).TryOpen(dsl.UnreachableURL(t))
	if err == nil || !strings.Contains(err.Error(), "ERR_CONNECTION_REFUSED") {
		t.Errorf("err = %v, want connection refused", err)
	}
}

func TestOpenWithoutURLFails(t *testing.T) {
	t.Parallel()
	err := dsl.NewAgent(t).TryOpen("")
	if err == nil || !strings.Contains(err.Error(), "url is required") {
		t.Errorf("err = %v, want url is required", err)
	}
}
