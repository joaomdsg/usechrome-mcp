package chrome

import (
	"errors"
	"iter"
	"maps"
	"slices"
	"testing"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/css"

	"github.com/joaomdsg/usechrome-mcp/internal/browser"
)

// stylesHeaders stands in for Chrome's event stream, where pulling one event
// too many blocks until another stylesheet loads; hence the count. An empty
// id stands in for the stream failing.
func stylesHeaders(pulled *int, ids ...cdp.StyleSheetID) iter.Seq2[css.EventStyleSheetAdded, error] {
	return func(yield func(css.EventStyleSheetAdded, error) bool) {
		for _, id := range ids {
			*pulled++
			if id == "" {
				if !yield(css.EventStyleSheetAdded{}, errors.New("closed")) {
					return
				}
				continue
			}
			if !yield(css.EventStyleSheetAdded{Header: &css.StyleSheetHeader{StyleSheetID: id}}, nil) {
				return
			}
		}
	}
}

func TestSheetHeadersReadsOnlyUntilEveryNeededSheetArrived(t *testing.T) {
	for name, tc := range map[string]struct {
		need   []cdp.StyleSheetID
		events []cdp.StyleSheetID
		want   []cdp.StyleSheetID
		pulled int
	}{
		"none needed reads nothing": {nil, []cdp.StyleSheetID{"a"}, nil, 0},
		"one needed":                {[]cdp.StyleSheetID{"a"}, []cdp.StyleSheetID{"x", "a", "b"}, []cdp.StyleSheetID{"a"}, 2},
		"two needed":                {[]cdp.StyleSheetID{"a", "b"}, []cdp.StyleSheetID{"b", "x", "a", "c"}, []cdp.StyleSheetID{"a", "b"}, 3},
		"error ends the wait":       {[]cdp.StyleSheetID{"a"}, []cdp.StyleSheetID{"", "a"}, nil, 1},
		"missing sheet":             {[]cdp.StyleSheetID{"a", "b"}, []cdp.StyleSheetID{"a", "x"}, []cdp.StyleSheetID{"a"}, 2},
	} {
		need := map[cdp.StyleSheetID]bool{}
		for _, id := range tc.need {
			need[id] = true
		}
		pulled := 0
		got := sheetHeaders(stylesHeaders(&pulled, tc.events...), need)
		ids := slices.Sorted(maps.Keys(got))
		if got == nil || !slices.Equal(ids, tc.want) || pulled != tc.pulled {
			t.Errorf("%s: sheetHeaders = %v (nil %v) after %d events; want %v after %d", name, ids, got == nil, pulled, tc.want, tc.pulled)
		}
		for id, h := range got {
			if h == nil || h.StyleSheetID != id {
				t.Errorf("%s: got[%q] = %+v; want its header", name, id, h)
			}
		}
	}
}

func TestSourceGivesSelectorLineOrBareURL(t *testing.T) {
	sheet := &css.StyleSheetHeader{SourceURL: "https://a.test/s.css", StartLine: 10}
	ranged := []*css.Value{{Text: "p", Range: &css.SourceRange{StartLine: 2}}}
	unranged := []*css.Value{{Text: "p"}}
	for name, tc := range map[string]struct {
		h    *css.StyleSheetHeader
		sel  []*css.Value
		want string
	}{
		"no header":         {nil, ranged, ""},
		"no url":            {&css.StyleSheetHeader{StartLine: 10}, ranged, ""},
		"selector range":    {sheet, ranged, "https://a.test/s.css:13"},
		"no selector range": {sheet, unranged, "https://a.test/s.css"},
		"no selectors":      {sheet, nil, "https://a.test/s.css"},
	} {
		r := &css.Rule{SelectorList: &css.SelectorList{Selectors: tc.sel}}
		if got := source(tc.h, r); got != tc.want {
			t.Errorf("%s: source = %q; want %q", name, got, tc.want)
		}
	}
}

func TestDeclarationsSkipsExpandedAndDisabledAndMarksImportant(t *testing.T) {
	at := &css.SourceRange{}
	got := declarations(&css.Style{CSSProperties: []*css.Property{
		{Name: "margin-top", Value: "0"},
		{Name: "color", Value: "red", Range: at},
		{Name: "width", Value: "1px", Range: at, Disabled: true},
		{Name: "gap", Value: "2px", Range: at, Important: true},
		{Name: "top", Value: "0 !important", Range: at, Important: true},
	}})
	want := []string{"color: red", "gap: 2px !important", "top: 0 !important"}
	if !slices.Equal(got, want) {
		t.Errorf("declarations = %q; want %q", got, want)
	}
	if got := declarations(nil); got != nil {
		t.Errorf("declarations(nil) = %q; want nil", got)
	}
}

func TestDeclaredPropertiesSortsAndDropsDuplicates(t *testing.T) {
	got := declaredProperties([]browser.Rule{
		{Declarations: []string{"width: 1px", "gap: 2px"}},
		{Declarations: []string{"color: red", "gap: 3px"}},
	})
	want := append(slices.Clone(basicProperties), "gap")
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("declaredProperties = %q; want %q", got, want)
	}
}
