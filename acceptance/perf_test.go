//go:build acceptance

package acceptance_test

import (
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/joaomdsg/usechrome-mcp/acceptance/dsl"
)

func TestAgentFindsTheCodeBehindASlowClick(t *testing.T) {
	t.Parallel()
	site := dsl.NewSite(t)
	agent := dsl.NewAgent(t)
	agent.Open(site.URL("/checkout.html"))
	agent.Click(dsl.CSS("#pay"))
	report := agent.Perf("click on button#pay")
	// A loaded runner adds its own delay to the 300ms, which can tip it past
	// the 500ms line into poor.
	if !regexp.MustCompile(`\nINP \d+ms \((needs improvement|poor)\) over 1 interaction, the slowest:\n`).MatchString(report) {
		t.Errorf("perf after a 300ms click:\n%s\nwant INP rated needs improvement or poor", report)
	}
	blame := regexp.MustCompile(`event-listener BUTTON#pay\.onclick function checkout in (\S+) at char (\d+) of its script`).FindStringSubmatch(report)
	if blame == nil || blame[1] != site.URL("/checkout.js") {
		t.Fatalf("perf after a 300ms click:\n%s\nwant the frame that ran checkout from checkout.js", report)
	}
	js := fixture(t, "checkout.js")
	char, _ := strconv.Atoi(blame[2])
	if at := strings.Index(js, "function checkout"); char < at || char > at+len("function checkout()") {
		t.Errorf("char %d, want where function checkout starts, at %d", char, at)
	}
	if code := agent.Source(blame[1], char); !strings.Contains(code, "while (performance.now() < end) {}") {
		t.Errorf("source from char %d:\n%s\nwant the busy loop", char, code)
	}
}

func TestPerfRatesTheLoadOfAPage(t *testing.T) {
	t.Parallel()
	site := dsl.NewSite(t)
	agent := dsl.NewAgent(t)
	agent.Open(site.URL("/"))
	report := agent.Perf("(good): h1\n")
	for _, want := range []string{
		site.URL("/") + ", since it began to load:\n",
		"\nTTFB ", "\nFCP ", "\nLCP ",
		"\nCLS 0.000 (good): no layout shifts\n",
		"\nINP none yet: no interactions\n",
		"\n8 elements; the renderer's JS heap uses ",
	} {
		if !strings.Contains(report, want) {
			t.Errorf("perf of the home page:\n%s\nwant %q", report, want)
		}
	}
	// A loaded runner can drop a frame even on this page, so either count passes.
	if !regexp.MustCompile(`\n(no|\d+) long animation frames`).MatchString(report) {
		t.Errorf("perf of the home page:\n%s\nwant the long animation frames section", report)
	}
	if n := strings.Count(report, "ms (good)"); n != 3 {
		t.Errorf("perf of the home page:\n%s\nwant TTFB, FCP and LCP rated good", report)
	}
}
