package browser

// StylesRequest asks for the CSS of the element that Ref, a ref from a
// snapshot, or Selector names. Properties lists the computed values to
// read; empty means those the element's rules set plus layout basics.
type StylesRequest struct {
	Tab        string
	Ref        string
	Selector   string
	Properties []string
}

// Styles is an element's CSS as DevTools shows it. Rules are the page's
// rules that match the element, highest priority first, with the inline
// style first of all.
type Styles struct {
	Element  string
	Box      Box
	Computed map[string]string
	Rules    []Rule
}

// Box is an element's border box in CSS pixels, relative to the viewport.
type Box struct {
	X, Y, Width, Height float64
}

// Rule is a CSS rule. Source is the stylesheet URL and line, empty for the
// inline style. Media is the media query the rule sits under, if any.
type Rule struct {
	Selector     string
	Media        string
	Source       string
	Declarations []string
}
