package chrome

import (
	"context"
	"encoding/json"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"

	"github.com/joaomdsg/usechrome-mcp/internal/browser"
)

// perfJS reads the page's performance timeline into browser.Perf's fields,
// times in nanoseconds. Observing with buffered set hands over the entries
// the browser kept from the start, and takeRecords hands them over now
// rather than on a later task. An interaction lasts as long as its longest
// event, as INP counts it, and splits into phases as web-vitals splits it:
// from its first event to the first handler, from there to the last
// handler's end, and from there to the next frame. It goes by the event
// whose handlers ran longest, so a click reads as "click", not as the
// "pointerdown" that came first but ran nothing. It returns a string because
// JSON.stringify writes whole numbers without an exponent, which a
// time.Duration needs.
const perfJS = `(() => {
	const ns = ms => Math.round(ms * 1e6);
	const kept = type => {
		const o = new PerformanceObserver(() => {});
		try { o.observe({type, buffered: true}); } catch { return []; }
		const r = o.takeRecords();
		o.disconnect();
		return r;
	};
	const describe = node => {
		const el = node?.nodeType === Node.ELEMENT_NODE ? node : node?.parentElement;
		if (!el) return "";
		if (el.id) return el.localName + "#" + el.id;
		return [el.localName, ...[...el.classList].slice(0, 2)].join(".");
	};
	const handlers = e => e.processingEnd - e.processingStart;
	const events = Map.groupBy(kept("event").filter(e => e.interactionId), e => e.interactionId);
	const interaction = es => {
		const start = Math.min(...es.map(e => e.startTime));
		const end = start + Math.max(...es.map(e => e.duration));
		const handling = Math.min(...es.map(e => e.processingStart));
		const handled = Math.max(...es.map(e => e.processingEnd));
		const main = es.reduce((a, e) => handlers(e) > handlers(a) ? e : a);
		return {
			Event: main.name, Target: describe(main.target ?? es.find(e => e.target)?.target),
			Start: ns(start), Duration: ns(end - start), InputDelay: ns(handling - start),
			Processing: ns(handled - handling), Presentation: ns(Math.max(0, end - handled)),
		};
	};
	const nav = performance.getEntriesByType("navigation")[0];
	const lcp = kept("largest-contentful-paint").at(-1);
	return JSON.stringify({
		URL: location.href,
		TTFB: ns(nav?.responseStart ?? 0),
		FCP: ns(performance.getEntriesByName("first-contentful-paint")[0]?.startTime ?? 0),
		LCP: ns(lcp?.startTime ?? 0),
		LCPElement: [describe(lcp?.element), lcp?.url].filter(Boolean).join(" "),
		DOMContentLoaded: ns(nav?.domContentLoadedEventStart ?? 0),
		Load: ns(nav?.loadEventStart ?? 0),
		Shifts: kept("layout-shift").map(e => ({
			Start: ns(e.startTime), Value: e.value, Input: e.hadRecentInput,
			Moved: e.sources.map(s => describe(s.node)).filter(Boolean),
		})),
		InteractionCount: performance.interactionCount,
		Interactions: [...events.values()].map(interaction),
		Frames: kept("long-animation-frame").map(f => ({
			Start: ns(f.startTime), Duration: ns(f.duration), Blocking: ns(f.blockingDuration),
			Scripts: f.scripts.map(s => ({
				Invoker: s.invoker, Kind: s.invokerType, Function: s.sourceFunctionName,
				URL: s.sourceURL, Char: s.sourceCharPosition, Duration: ns(s.duration),
			})),
		})),
		Elements: document.getElementsByTagName("*").length,
	});
})()`

// Perf reads what the browser measured on a tab's page. The timeline keeps
// its entries from the start of the load, so nothing has to record ahead.
func (b *Browser) Perf(ctx context.Context, req browser.PerfRequest) (browser.Perf, error) {
	var p browser.Perf
	err := b.do(ctx, req.Tab, func(tctx context.Context) error {
		js, err := chromedp.Run(tctx, chromedp.Evaluate[string](perfJS))
		if err != nil {
			return err
		}
		if err := json.Unmarshal([]byte(js), &p); err != nil {
			return err
		}
		heap, err := chromedp.Call(tctx, runtime.GetHeapUsage, cdp.Empty{})
		if err != nil {
			return err
		}
		p.HeapUsed, p.HeapTotal = int(heap.UsedSize), int(heap.TotalSize)
		return nil
	})
	return p, err
}
