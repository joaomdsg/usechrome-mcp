package chrome_test

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/joaomdsg/usechrome-mcp/internal/browser"
	"github.com/joaomdsg/usechrome-mcp/internal/chrome"
)

// perfPage blocks its first frame for 120 ms, pushes its text down once it
// has painted, and makes a click on its button take 250 ms.
const perfPage = `<!doctype html><title>Perf</title>
<h1>A heading big enough to be the largest paint</h1>
<p id="anchor">pushed down</p>
<button id="slow">Slow</button>
<script>
const busy = ms => { const end = performance.now() + ms; while (performance.now() < end) {} };
busy(120);
requestAnimationFrame(() => setTimeout(() => {
	const ad = document.createElement("div");
	ad.style.height = "200px";
	document.body.insertBefore(ad, document.getElementById("anchor"));
}, 100));
document.getElementById("slow").addEventListener("click", function buy() { busy(250) });
</script>`

// eventTimingStep is how finely the browser rounds an event's duration:
// "rounded to the nearest 8ms" (w3c.github.io/event-timing, 2.1).
const eventTimingStep = 8 * time.Millisecond

func perfSite(t *testing.T) string {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/{$}", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(perfPage)) })
	mux.HandleFunc("/favicon.ico", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv.URL + "/"
}

func waitPerf(t *testing.T, b *chrome.Browser, done func(browser.Perf) bool) browser.Perf {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		p, err := b.Perf(t.Context(), browser.PerfRequest{})
		if err != nil {
			t.Fatalf("perf: %v", err)
		}
		if done(p) || time.Now().After(deadline) {
			return p
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestPerfTimesTheLoadInOrder(t *testing.T) {
	t.Parallel()
	b := newBrowser(t)
	url := perfSite(t)
	nav(t, b, browser.NavRequest{URL: url})
	p := waitPerf(t, b, func(p browser.Perf) bool { return p.LCP > 0 })
	if p.URL != url || p.LCPElement != "h1" {
		t.Errorf("perf of %s, largest paint %q; want %s, h1", p.URL, p.LCPElement, url)
	}
	steps := []time.Duration{0, p.TTFB, p.DOMContentLoaded, p.Load}
	if !slices.IsSorted(steps) || slices.Contains(steps[1:], 0) || p.FCP < 120*time.Millisecond || p.LCP < p.FCP {
		t.Errorf("TTFB %v, DOMContentLoaded %v, load %v, FCP %v, LCP %v; want each later than the one before, and the paints after the 120ms the script blocks them",
			p.TTFB, p.DOMContentLoaded, p.Load, p.FCP, p.LCP)
	}
	if p.HeapUsed <= 0 || p.HeapTotal < p.HeapUsed {
		t.Errorf("JS heap %d of %d bytes, want some used of a larger total", p.HeapUsed, p.HeapTotal)
	}
}

func TestPerfFindsWhatTheShiftMoved(t *testing.T) {
	t.Parallel()
	b := newBrowser(t)
	nav(t, b, browser.NavRequest{URL: perfSite(t)})
	p := waitPerf(t, b, func(p browser.Perf) bool { return len(p.Shifts) > 0 })
	if len(p.Shifts) != 1 || p.Shifts[0].Value <= 0 || p.Shifts[0].Input || !slices.Contains(p.Shifts[0].Moved, "p#anchor") {
		t.Fatalf("shifts = %+v, want one that moved p#anchor without input", p.Shifts)
	}
	// html, head, title, body, h1, p, button, script and the inserted div.
	if p.Elements != 9 {
		t.Errorf("elements = %d, want 9", p.Elements)
	}
}

// clickSlow clicks the slow button of a fresh perfPage at url and waits
// until the browser has reported the click.
func clickSlow(t *testing.T, url string) browser.Perf {
	t.Helper()
	b := newBrowser(t)
	nav(t, b, browser.NavRequest{URL: url})
	if p := waitPerf(t, b, func(browser.Perf) bool { return true }); p.InteractionCount != 0 || len(p.Interactions) != 0 {
		t.Fatalf("before any input: %d interactions, kept %+v; want none", p.InteractionCount, p.Interactions)
	}
	if _, err := b.Interact(t.Context(), browser.InteractRequest{Action: browser.Click, Selector: "#slow"}); err != nil {
		t.Fatal(err)
	}
	return waitPerf(t, b, func(p browser.Perf) bool { return len(p.Interactions) > 0 && handlerFrame(p) != nil })
}

func TestPerfTimesASlowClickByItsHandler(t *testing.T) {
	t.Parallel()
	p := clickSlow(t, perfSite(t))
	if len(p.Interactions) != 1 || p.InteractionCount != 1 {
		t.Fatalf("%d interactions, kept %+v; want the click", p.InteractionCount, p.Interactions)
	}
	in := p.Interactions[0]
	if in.Event != "click" || in.Target != "button#slow" || in.Processing < 250*time.Millisecond || in.Duration < in.Processing-eventTimingStep {
		t.Errorf("interaction = %+v, want a click on button#slow, 250ms or more in its handler", in)
	}
}

func TestPerfBlamesTheHandlerOfASlowClick(t *testing.T) {
	t.Parallel()
	url := perfSite(t)
	p := clickSlow(t, url)
	got := handlerFrame(p)
	if got == nil || got.Function != "buy" || got.Kind != "event-listener" || got.URL != url || got.Duration < 250*time.Millisecond {
		t.Errorf("frames = %+v, want one with buy from %s running 250ms or more", p.Frames, url)
	}
}

func handlerFrame(p browser.Perf) *browser.Script {
	for _, f := range p.Frames {
		for _, s := range f.Scripts {
			if s.Invoker == "BUTTON#slow.onclick" {
				return &s
			}
		}
	}
	return nil
}
