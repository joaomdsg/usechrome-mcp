package chrome

import (
	"context"
	"fmt"
	"iter"
	"slices"
	"strings"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/css"
	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"

	"github.com/joaomdsg/usechrome-mcp/internal/browser"
)

// basicProperties are read on top of what an element's rules set, as the
// box model and text facts a person checks first.
var basicProperties = []string{
	"display", "position", "box-sizing", "width", "height", "margin", "padding", "border",
	"color", "background-color", "font", "opacity", "visibility", "z-index", "overflow",
}

// headerWait bounds the wait for stylesheet headers. CSS.enable sends them
// all before it returns, so by then they only sit in our event queue.
const headerWait = time.Second

// Styles reads an element's matched CSS rules and computed values.
func (b *Browser) Styles(ctx context.Context, req browser.StylesRequest) (browser.Styles, error) {
	var out browser.Styles
	err := b.do(ctx, req.Tab, func(tctx context.Context) error {
		defer releaseElements(tctx)
		el, err := element(tctx, req.Ref, req.Selector)
		if err != nil {
			return err
		}
		rules, err := matchedRules(tctx, el)
		if err != nil {
			return err
		}
		props := req.Properties
		if len(props) == 0 {
			props = declaredProperties(rules)
		}
		out, err = callOn[browser.Styles](tctx, el, stylesJS, props)
		out.Rules = rules
		return err
	})
	return out, err
}

const stylesJS = `function(props) {
	const r = this.getBoundingClientRect();
	const cs = getComputedStyle(this);
	const computed = {};
	for (const p of props) computed[p] = cs.getPropertyValue(p);
	return {
		Element: this.localName + (this.id ? "#" + this.id : "") + [...this.classList].map(c => "." + c).join(""),
		Box: {X: r.x, Y: r.y, Width: r.width, Height: r.height},
		Computed: computed,
	};
}`

// matchedRules lists the page's rules that match el, highest priority
// first. Stylesheet headers are the only place CDP gives a stylesheet's
// URL, and CSS.enable sends them only when the domain was off; chromedp
// turns it on with each tab, so it is turned off and on again here.
func matchedRules(ctx context.Context, el runtime.RemoteObjectID) ([]browser.Rule, error) {
	hctx, cancel := context.WithTimeout(ctx, headerWait)
	defer cancel()
	headers := chromedp.Events(hctx, css.StyleSheetAdded)
	if _, err := chromedp.Call(ctx, css.Disable, cdp.Empty{}); err != nil {
		return nil, err
	}
	if _, err := chromedp.Call(ctx, css.Enable, cdp.Empty{}); err != nil {
		return nil, err
	}
	node, err := chromedp.Call(ctx, dom.RequestNode, dom.RequestNodeParams{ObjectID: el})
	if err != nil {
		return nil, err
	}
	m, err := chromedp.Call(ctx, css.GetMatchedStylesForNode, css.GetMatchedStylesForNodeParams{NodeID: node.NodeID})
	if err != nil {
		return nil, err
	}
	var authored []*css.Rule
	need := map[cdp.StyleSheetID]bool{}
	for _, match := range slices.Backward(m.MatchedCSSRules) {
		if r := match.Rule; r.Origin == css.StyleSheetOriginRegular {
			authored = append(authored, r)
			need[r.StyleSheetID] = true
		}
	}
	sheets := sheetHeaders(headers, need)
	var rules []browser.Rule
	if s := declarations(m.InlineStyle); len(s) > 0 {
		rules = append(rules, browser.Rule{Selector: "style attribute", Declarations: s})
	}
	for _, r := range authored {
		rules = append(rules, browser.Rule{
			Selector:     r.SelectorList.Text,
			Media:        media(r.Media),
			Source:       source(sheets[r.StyleSheetID], r),
			Declarations: declarations(r.Style),
		})
	}
	return rules, nil
}

// sheetHeaders reads headers until it has those of the needed stylesheets.
// A missing one leaves its rules without a source once the wait ends.
func sheetHeaders(headers iter.Seq2[css.EventStyleSheetAdded, error], need map[cdp.StyleSheetID]bool) map[cdp.StyleSheetID]*css.StyleSheetHeader {
	got := map[cdp.StyleSheetID]*css.StyleSheetHeader{}
	if len(need) == 0 {
		return got
	}
	for e, err := range headers {
		if err != nil {
			break
		}
		if need[e.Header.StyleSheetID] {
			got[e.Header.StyleSheetID] = e.Header
		}
		if len(got) == len(need) {
			break
		}
	}
	return got
}

// source gives the URL and line of a rule's selector.
func source(h *css.StyleSheetHeader, r *css.Rule) string {
	if h == nil || h.SourceURL == "" {
		return ""
	}
	if sel := r.SelectorList.Selectors; len(sel) > 0 && sel[0].Range != nil {
		return fmt.Sprintf("%s:%d", h.SourceURL, int64(h.StartLine)+sel[0].Range.StartLine+1)
	}
	return h.SourceURL
}

func media(list []*css.Media) string {
	texts := make([]string, 0, len(list))
	for _, m := range list {
		texts = append(texts, m.Text)
	}
	return strings.Join(texts, " and ")
}

// declarations lists what the author wrote, without the longhands the
// browser expands shorthands into, which have no source range.
func declarations(s *css.Style) []string {
	if s == nil {
		return nil
	}
	var out []string
	for _, p := range s.CSSProperties {
		if p.Range == nil || p.Disabled {
			continue
		}
		d := p.Name + ": " + p.Value
		if p.Important && !strings.HasSuffix(p.Value, "!important") {
			d += " !important"
		}
		out = append(out, d)
	}
	return out
}

// declaredProperties is every property the rules set, then the basics.
func declaredProperties(rules []browser.Rule) []string {
	var props []string
	for _, r := range rules {
		for _, d := range r.Declarations {
			props = append(props, d[:strings.Index(d, ":")])
		}
	}
	props = append(props, basicProperties...)
	slices.Sort(props)
	return slices.Compact(props)
}
