package tools

import (
	"cmp"
	"context"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/joaomdsg/usechrome-mcp/internal/browser"
)

// maxListed caps each list of the perf report; the largest items stay.
const maxListed = 5

// limits rates a measure as Core Web Vitals do: good up to the first,
// poor past the second.
type limits [2]float64

// From web.dev/articles/ttfb, /fcp, /lcp, /cls and /inp.
var (
	ttfbLimits = limits{800, 1800}
	fcpLimits  = limits{1800, 3000}
	lcpLimits  = limits{2500, 4000}
	clsLimits  = limits{0.1, 0.25}
	inpLimits  = limits{200, 500}
)

func (l limits) rate(v float64) string {
	switch {
	case v <= l[0]:
		return "good"
	case v <= l[1]:
		return "needs improvement"
	}
	return "poor"
}

// The browser keeps an interaction only from this long: "the first
// multiple of 8 greater than 100ms" (w3c.github.io/event-timing, 4).
const keptInteraction = 104 * time.Millisecond

// Perfer reads how a tab's page has performed.
type Perfer interface {
	Perf(ctx context.Context, req browser.PerfRequest) (browser.Perf, error)
}

// PerfInput is the arguments of the perf tool.
type PerfInput struct {
	Target
}

// AddPerf registers the perf tool, which drives p.
func AddPerf(s *mcp.Server, p Perfer) {
	tool := &mcp.Tool{
		Name: "perf",
		Description: "Measure a tab's current page since it began to load: TTFB, FCP, LCP, CLS and INP rated by the Core Web Vitals thresholds, " +
			"the element of the largest paint, the elements that layout shifts moved, the slowest interactions split into input delay, handlers and rendering, " +
			"the long animation frames with the scripts that ran in them, the page's element count and the JS heap. " +
			"Interact with the page first to measure how fast it responds.",
		InputSchema: inputSchema[PerfInput](),
	}
	mcp.AddTool(s, tool, func(ctx context.Context, _ *mcp.CallToolRequest, args PerfInput) (*mcp.CallToolResult, any, error) {
		got, err := p.Perf(ctx, browser.PerfRequest{Tab: args.Tab})
		if err != nil {
			return nil, nil, err
		}
		return textResult(cutText(renderPerf(got), maxTextChars)), nil, nil
	})
}

func renderPerf(p browser.Perf) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s, since it began to load:\n\n", p.URL)
	b.WriteString(vital("TTFB", p.TTFB, ttfbLimits) + "\n")
	b.WriteString(vital("FCP", p.FCP, fcpLimits) + "\n")
	b.WriteString(vital("LCP", p.LCP, lcpLimits))
	if p.LCPElement != "" {
		b.WriteString(": " + cutText(p.LCPElement, maxListedURLChars))
	}
	fmt.Fprintf(&b, "\nDOMContentLoaded %s, load %s\n", moment(p.DOMContentLoaded), moment(p.Load))
	writeShifts(&b, p.Shifts)
	writeInteractions(&b, p)
	writeFrames(&b, p.Frames)
	fmt.Fprintf(&b, "\n%d elements; the renderer's JS heap uses %s of %s\n", p.Elements, byteSize(p.HeapUsed), byteSize(p.HeapTotal))
	return b.String()
}

func vital(name string, d time.Duration, l limits) string {
	if d == 0 {
		return name + " none yet"
	}
	return fmt.Sprintf("%s %s (%s)", name, ms(d), l.rate(float64(d.Milliseconds())))
}

func moment(d time.Duration) string {
	if d == 0 {
		return "none yet"
	}
	return ms(d)
}

func ms(d time.Duration) string {
	return fmt.Sprintf("%dms", d.Milliseconds())
}

func writeShifts(b *strings.Builder, shifts []browser.Shift) {
	worst := worstBurst(shifts)
	score := 0.0
	for _, s := range worst {
		score += s.Value
	}
	// Rated as shown, so 0.1004 reads "0.100 (good)".
	score = math.Round(score*1000) / 1000
	fmt.Fprintf(b, "\nCLS %.3f (%s)", score, clsLimits.rate(score))
	if len(worst) == 0 {
		b.WriteString(": no layout shifts\n")
		return
	}
	b.WriteString(", the worst burst of layout shifts:\n")
	list, more := largest(worst, func(s browser.Shift) float64 { return s.Value })
	for _, s := range list {
		moved := cmp.Or(strings.Join(s.Moved, ", "), "elements since removed")
		fmt.Fprintf(b, "  %.3f at %s moved %s\n", s.Value, ms(s.Start), moved)
	}
	writeMore(b, "  ", more)
}

// worstBurst is the burst of layout shifts that CLS scores, as web-vitals
// finds it: a burst takes shifts under 1 s apart for under 5 s, and leaves
// out those right after input.
func worstBurst(shifts []browser.Shift) []browser.Shift {
	var worst, burst []browser.Shift
	worstScore, burstScore := 0.0, 0.0
	for _, s := range shifts {
		if s.Input {
			continue
		}
		if len(burst) > 0 && (s.Start-burst[len(burst)-1].Start >= time.Second || s.Start-burst[0].Start >= 5*time.Second) {
			burst, burstScore = nil, 0
		}
		burst = append(burst, s)
		burstScore += s.Value
		if burstScore > worstScore {
			worst, worstScore = burst, burstScore
		}
	}
	return worst
}

func writeInteractions(b *strings.Builder, p browser.Perf) {
	if p.InteractionCount == 0 {
		b.WriteString("\nINP none yet: no interactions\n")
		return
	}
	count := fmt.Sprintf("%d interaction", p.InteractionCount)
	if p.InteractionCount > 1 {
		count += "s"
	}
	if d, ok := inp(p); ok {
		fmt.Fprintf(b, "\nINP %s (%s) over %s", ms(d), inpLimits.rate(float64(d.Milliseconds())), count)
	} else {
		fmt.Fprintf(b, "\nINP under %s (good) over %s", ms(keptInteraction), count)
	}
	if len(p.Interactions) == 0 {
		fmt.Fprintf(b, ", none of them %s or longer\n", ms(keptInteraction))
		return
	}
	b.WriteString(", the slowest:\n")
	list, more := largest(p.Interactions, func(in browser.InteractionTiming) float64 { return float64(in.Duration) })
	for _, in := range list {
		fmt.Fprintf(b, "  %s %s on %s at %s: %s input delay, %s handlers, %s rendering\n",
			ms(in.Duration), in.Event, cmp.Or(in.Target, "an element since removed"), ms(in.Start),
			ms(in.InputDelay), ms(in.Processing), ms(in.Presentation))
	}
	writeMore(b, "  ", more)
}

// inp is the slowest interaction but one for every 50, as web-vitals
// counts it, so a long session forgives a few outliers. The browser keeps
// only the slow interactions, so when the one INP falls on is not kept, it
// was faster than those, and INP is unknown but good.
func inp(p browser.Perf) (time.Duration, bool) {
	durations := make([]time.Duration, 0, len(p.Interactions))
	for _, in := range p.Interactions {
		durations = append(durations, in.Duration)
	}
	slices.SortFunc(durations, func(a, b time.Duration) int { return cmp.Compare(b, a) })
	if i := p.InteractionCount / 50; i < len(durations) {
		return durations[i], true
	}
	return 0, false
}

func writeFrames(b *strings.Builder, frames []browser.Frame) {
	if len(frames) == 0 {
		b.WriteString("\nno long animation frames\n")
		return
	}
	var blocking time.Duration
	for _, f := range frames {
		blocking += f.Blocking
	}
	fmt.Fprintf(b, "\n%d long animation frames, %s blocking in all, the slowest:\n", len(frames), ms(blocking))
	list, more := largest(frames, func(f browser.Frame) float64 { return float64(f.Duration) })
	for _, f := range list {
		fmt.Fprintf(b, "  %s at %s, %s blocking\n", ms(f.Duration), ms(f.Start), ms(f.Blocking))
		scripts, cut := largest(f.Scripts, func(s browser.Script) float64 { return float64(s.Duration) })
		for _, s := range scripts {
			b.WriteString("    " + scriptLine(s) + "\n")
		}
		writeMore(b, "    ", cut)
	}
	writeMore(b, "  ", more)
}

func scriptLine(s browser.Script) string {
	parts := []string{ms(s.Duration), s.Kind, cutText(s.Invoker, maxListedURLChars)}
	if s.Function != "" {
		parts = append(parts, "function "+s.Function)
	}
	if s.URL != "" && s.URL != s.Invoker {
		parts = append(parts, "in "+cutText(s.URL, maxListedURLChars))
	}
	// A whole script, not a function in it, starts at 0.
	if s.Char > 0 {
		parts = append(parts, fmt.Sprintf("at char %d of its script", s.Char))
	}
	return strings.Join(parts, " ")
}

// largest returns the maxListed largest items, largest first, and how many
// it left out.
func largest[T any](items []T, size func(T) float64) ([]T, int) {
	sorted := slices.SortedStableFunc(slices.Values(items), func(a, b T) int { return cmp.Compare(size(b), size(a)) })
	n := min(len(sorted), maxListed)
	return sorted[:n], len(sorted) - n
}

func writeMore(b *strings.Builder, indent string, n int) {
	if n > 0 {
		fmt.Fprintf(b, "%s... %d more\n", indent, n)
	}
}
