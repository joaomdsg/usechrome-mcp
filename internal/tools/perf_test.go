package tools_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/joaomdsg/usechrome-mcp/internal/browser"
	"github.com/joaomdsg/usechrome-mcp/internal/tools"
)

type fakePerfer struct {
	got  []browser.PerfRequest
	perf browser.Perf
	err  error
}

func (f *fakePerfer) Perf(_ context.Context, req browser.PerfRequest) (browser.Perf, error) {
	f.got = append(f.got, req)
	return f.perf, f.err
}

func perfText(t *testing.T, p browser.Perf) string {
	t.Helper()
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	tools.AddPerf(srv, &fakePerfer{perf: p})
	res := call(t, connect(t, srv), "perf", map[string]any{})
	if res.IsError {
		t.Fatalf("perf failed: %s", errorText(res))
	}
	return errorText(res)
}

const millis = time.Millisecond

func TestPerfWritesTheReport(t *testing.T) {
	t.Parallel()
	f := &fakePerfer{perf: browser.Perf{
		URL: "http://x/shop", TTFB: 120 * millis, FCP: 900 * millis, LCP: 2600 * millis, LCPElement: "img.hero http://x/hero.jpg",
		DOMContentLoaded: 1000 * millis, Load: 3000 * millis,
		Shifts:           []browser.Shift{{Start: 1200 * millis, Value: 0.15, Moved: []string{"div.ad", "p#total"}}},
		InteractionCount: 1,
		Interactions: []browser.InteractionTiming{{
			Event: "click", Target: "button#buy", Start: 5000 * millis, Duration: 312 * millis,
			InputDelay: 4 * millis, Processing: 300 * millis, Presentation: 8 * millis,
		}},
		Frames: []browser.Frame{
			{Start: 400 * millis, Duration: 125 * millis, Blocking: 74 * millis, Scripts: []browser.Script{
				{Invoker: "http://x/app.js", Kind: "classic-script", URL: "http://x/app.js", Duration: 121 * millis},
			}},
			{Start: 5000 * millis, Duration: 312 * millis, Blocking: 262 * millis, Scripts: []browser.Script{
				{Invoker: "BUTTON#buy.onclick", Kind: "event-listener", Function: "buy", URL: "http://x/app.js", Char: 347, Duration: 300 * millis},
			}},
		},
		Elements: 120, HeapUsed: 2_500_000, HeapTotal: 4_000_000,
	}}
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	tools.AddPerf(srv, f)
	got := errorText(call(t, connect(t, srv), "perf", map[string]any{"tab": "t2"}))
	want := `http://x/shop, since it began to load:

TTFB 120ms (good)
FCP 900ms (good)
LCP 2600ms (needs improvement): img.hero http://x/hero.jpg
DOMContentLoaded 1000ms, load 3000ms

CLS 0.150 (needs improvement), the worst burst of layout shifts:
  0.150 at 1200ms moved div.ad, p#total

INP 312ms (needs improvement) over 1 interaction, the slowest:
  312ms click on button#buy at 5000ms: 4ms input delay, 300ms handlers, 8ms rendering

2 long animation frames, 336ms blocking in all, the slowest:
  312ms at 5000ms, 262ms blocking
    300ms event-listener BUTTON#buy.onclick function buy in http://x/app.js at char 347 of its script
  125ms at 400ms, 74ms blocking
    121ms classic-script http://x/app.js

120 elements; the renderer's JS heap uses 2.5MB of 4.0MB
`
	if got != want {
		t.Errorf("perf =\n%s\nwant\n%s", got, want)
	}
	if want := (browser.PerfRequest{Tab: "t2"}); len(f.got) != 1 || f.got[0] != want {
		t.Errorf("browser called with %+v, want %+v", f.got, want)
	}
}

func TestPerfSaysWhatThePageHasNotReachedYet(t *testing.T) {
	t.Parallel()
	got := perfText(t, browser.Perf{URL: "http://x/", TTFB: 5 * millis, Elements: 4, HeapUsed: 900_000, HeapTotal: 1_800_000})
	want := `http://x/, since it began to load:

TTFB 5ms (good)
FCP none yet
LCP none yet
DOMContentLoaded none yet, load none yet

CLS 0.000 (good): no layout shifts

INP none yet: no interactions

no long animation frames

4 elements; the renderer's JS heap uses 900.0kB of 1.8MB
`
	if got != want {
		t.Errorf("perf =\n%s\nwant\n%s", got, want)
	}
}

func slowClick(d time.Duration) []browser.InteractionTiming {
	return []browser.InteractionTiming{{Event: "click", Target: "button", Duration: d}}
}

func TestPerfRatesEachVitalByItsThresholds(t *testing.T) {
	t.Parallel()
	cases := []struct {
		perf browser.Perf
		want string
	}{
		{browser.Perf{TTFB: 800 * millis}, "TTFB 800ms (good)"},
		{browser.Perf{TTFB: 801 * millis}, "TTFB 801ms (needs improvement)"},
		{browser.Perf{TTFB: 1800 * millis}, "TTFB 1800ms (needs improvement)"},
		{browser.Perf{TTFB: 1801 * millis}, "TTFB 1801ms (poor)"},
		{browser.Perf{FCP: 1800 * millis}, "FCP 1800ms (good)"},
		{browser.Perf{FCP: 3001 * millis}, "FCP 3001ms (poor)"},
		{browser.Perf{LCP: 2500 * millis}, "LCP 2500ms (good)"},
		{browser.Perf{LCP: 4000 * millis}, "LCP 4000ms (needs improvement)"},
		{browser.Perf{LCP: 4001 * millis}, "LCP 4001ms (poor)"},
		{browser.Perf{Shifts: []browser.Shift{{Value: 0.1}}}, "CLS 0.100 (good)"},
		{browser.Perf{Shifts: []browser.Shift{{Value: 0.1004}}}, "CLS 0.100 (good)"},
		{browser.Perf{Shifts: []browser.Shift{{Value: 0.25}}}, "CLS 0.250 (needs improvement)"},
		{browser.Perf{Shifts: []browser.Shift{{Value: 0.251}}}, "CLS 0.251 (poor)"},
		{browser.Perf{InteractionCount: 1, Interactions: slowClick(200 * millis)}, "INP 200ms (good)"},
		{browser.Perf{InteractionCount: 1, Interactions: slowClick(500 * millis)}, "INP 500ms (needs improvement)"},
		{browser.Perf{InteractionCount: 1, Interactions: slowClick(501 * millis)}, "INP 501ms (poor)"},
	}
	for _, c := range cases {
		if got := perfText(t, c.perf); !strings.Contains(got, "\n"+c.want) {
			t.Errorf("perf of %+v =\n%s\nwant a line %q", c.perf, got, c.want)
		}
	}
}

func shiftsEvery(start, step time.Duration, n int, value float64) []browser.Shift {
	var out []browser.Shift
	for i := range n {
		out = append(out, browser.Shift{Start: start + time.Duration(i)*step, Value: value, Moved: []string{"p"}})
	}
	return out
}

func TestPerfScoresTheWorstBurstOfShifts(t *testing.T) {
	t.Parallel()
	shifts := []browser.Shift{
		{Start: 0, Value: 0.05, Moved: []string{"img"}},
		{Start: 900 * millis, Value: 0.06, Moved: []string{"img"}},
		{Start: 1500 * millis, Value: 0.5, Input: true, Moved: []string{"div.menu"}},
	}
	// Under 1 s apart for 5.4 s: the burst ends before the last one.
	shifts = append(shifts, shiftsEvery(3*time.Second, 900*millis, 7, 0.04)...)
	got := perfText(t, browser.Perf{Shifts: shifts})
	want := `
CLS 0.240 (needs improvement), the worst burst of layout shifts:
  0.040 at 3000ms moved p
  0.040 at 3900ms moved p
  0.040 at 4800ms moved p
  0.040 at 5700ms moved p
  0.040 at 6600ms moved p
  ... 1 more
`
	if !strings.Contains(got, want) {
		t.Errorf("perf =\n%s\nwant%s", got, want)
	}
}

func TestPerfForgivesOneSlowInteractionIn50(t *testing.T) {
	t.Parallel()
	var kept []browser.InteractionTiming
	for d := 900 * millis; d >= 300*millis; d -= 100 * millis {
		kept = append(kept, slowClick(d)...)
	}
	cases := []struct {
		count int
		want  string
	}{
		{49, "INP 900ms (poor) over 49 interactions, the slowest:"},
		{50, "INP 800ms (poor) over 50 interactions, the slowest:"},
		{300, "INP 300ms (needs improvement) over 300 interactions, the slowest:"},
		{350, "INP under 104ms (good) over 350 interactions, the slowest:"},
	}
	for _, c := range cases {
		if got := perfText(t, browser.Perf{InteractionCount: c.count, Interactions: kept}); !strings.Contains(got, "\n"+c.want+"\n") {
			t.Errorf("perf of %d interactions =\n%s\nwant a line %q", c.count, got, c.want)
		}
	}
}

func TestPerfSaysINPIsGoodWhenNoInteractionWasSlow(t *testing.T) {
	t.Parallel()
	got := perfText(t, browser.Perf{InteractionCount: 3})
	if want := "\nINP under 104ms (good) over 3 interactions, none of them 104ms or longer\n"; !strings.Contains(got, want) {
		t.Errorf("perf =\n%s\nwant%s", got, want)
	}
}

func TestPerfListsOnlyTheSlowestFramesAndScripts(t *testing.T) {
	t.Parallel()
	var frames []browser.Frame
	for i := range 7 {
		d := time.Duration(60+10*i) * millis
		f := browser.Frame{Start: time.Duration(i) * time.Second, Duration: d, Blocking: 10 * millis}
		for j := range i + 1 {
			f.Scripts = append(f.Scripts, browser.Script{Kind: "user-callback", Invoker: "TimerHandler:setTimeout", Duration: time.Duration(5+j) * millis})
		}
		frames = append(frames, f)
	}
	got := perfText(t, browser.Perf{Frames: frames})
	want := `
7 long animation frames, 70ms blocking in all, the slowest:
  120ms at 6000ms, 10ms blocking
    11ms user-callback TimerHandler:setTimeout
    10ms user-callback TimerHandler:setTimeout
    9ms user-callback TimerHandler:setTimeout
    8ms user-callback TimerHandler:setTimeout
    7ms user-callback TimerHandler:setTimeout
    ... 2 more
  110ms at 5000ms, 10ms blocking
`
	if !strings.Contains(got, want) || !strings.Contains(got, "  80ms at 2000ms, 10ms blocking\n") || strings.Contains(got, "at 1000ms") ||
		!strings.Contains(got, "  ... 2 more\n\n") {
		t.Errorf("perf =\n%s\nwant the 5 slowest frames, each with its 5 slowest scripts, starting%s", got, want)
	}
}

func TestPerfCountsOneLongAnimationFrameInTheSingular(t *testing.T) {
	t.Parallel()
	got := perfText(t, browser.Perf{Frames: []browser.Frame{{Duration: 60 * millis, Blocking: 10 * millis}}})
	if want := "\n1 long animation frame, 10ms blocking in all, the slowest:\n"; !strings.Contains(got, want) {
		t.Errorf("perf =\n%s\nwant%s", got, want)
	}
}

func TestPerfReportsBrowserError(t *testing.T) {
	t.Parallel()
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	tools.AddPerf(srv, &fakePerfer{err: errors.New("no tab t9")})
	res := call(t, connect(t, srv), "perf", map[string]any{"tab": "t9"})
	if !res.IsError || !strings.Contains(errorText(res), "no tab t9") {
		t.Errorf("result = %+v, want the browser error", res)
	}
}
