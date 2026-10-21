//go:build acceptance

package acceptance_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/joaomdsg/usechrome-mcp/acceptance/dsl"
)

func TestAgentReadsWhatAClickSentAndGotBack(t *testing.T) {
	t.Parallel()
	site := dsl.NewSite(t)
	agent := dsl.NewAgent(t)
	agent.Open(site.URL("/cart.html"))
	agent.Click(dsl.CSS("#add"))
	rows := agent.Requests("api/", 1)
	if len(rows) != 1 || rows[0].Method != "POST" || rows[0].Status != "200" || rows[0].Type != "fetch" {
		t.Fatalf("requests to the API = %+v, want the POST the click sent", rows)
	}
	detail := agent.Request(rows[0].ID)
	for _, want := range []string{
		"POST " + site.URL("/api/cart.json") + "\n",
		"  Content-Type: application/json\n",
		"request body (21B):\n" + `{"sku":"tea","qty":1}` + "\n",
		"response body (12B):\n" + `{"items":1}` + "\n",
	} {
		if !strings.Contains(detail, want) {
			t.Errorf("detail of the POST:\n%s\nwant it to contain %q", detail, want)
		}
	}
}

func TestNetworkListsWhatThePageLoaded(t *testing.T) {
	t.Parallel()
	site := dsl.NewSite(t)
	agent := dsl.NewAgent(t)
	agent.Open(site.URL("/shop.html"))
	want := []dsl.Row{
		{Method: "GET", Status: "200", Type: "document", Mime: "text/html", URL: site.URL("/shop.html")},
		{Method: "GET", Status: "200", Type: "script", Mime: "text/javascript", URL: site.URL("/shop.js")},
		{Method: "GET", Status: "404", Type: "image", Mime: "text/plain", URL: site.URL("/missing-logo.png")},
	}
	if got := summary(agent.Requests("", len(want))); !reflect.DeepEqual(got, want) {
		t.Errorf("requests of the shop:\n got %+v\nwant %+v", got, want)
	}
}

func TestNetworkShowsEachHopOfARedirect(t *testing.T) {
	t.Parallel()
	site := dsl.NewSite(t)
	agent := dsl.NewAgent(t)
	agent.Open(site.URL("/redirect"))
	want := []dsl.Row{
		{Method: "GET", Status: "302", Type: "document", Mime: "text/html", URL: site.URL("/redirect")},
		{Method: "GET", Status: "200", Type: "document", Mime: "text/html", URL: site.URL("/second.html")},
	}
	if got := summary(agent.Requests("", len(want))); !reflect.DeepEqual(got, want) {
		t.Errorf("requests of the redirect:\n got %+v\nwant %+v", got, want)
	}
}

func TestNetworkOfAnUnknownRequestFails(t *testing.T) {
	t.Parallel()
	site := dsl.NewSite(t)
	agent := dsl.NewAgent(t)
	agent.Open(site.URL("/index.html"))
	if err := agent.TryRequest("n999"); err == nil || !strings.Contains(err.Error(), "n999") {
		t.Errorf("detail of n999: %v, want an error naming it", err)
	}
}

// summary drops what varies between runs.
func summary(rows []dsl.Row) []dsl.Row {
	out := make([]dsl.Row, 0, len(rows))
	for _, r := range rows {
		out = append(out, dsl.Row{Method: r.Method, Status: r.Status, Type: r.Type, Mime: r.Mime, URL: r.URL})
	}
	return out
}
