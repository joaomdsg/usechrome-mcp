package chrome

import (
	"slices"
	"strings"
	"testing"

	"github.com/chromedp/chromedp"
)

func TestParseKeyReadsNamesCharactersAndModifiers(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]struct {
		r    rune
		mods []chromedp.Modifier
	}{
		"a":                 {'a', nil},
		"A":                 {'A', nil},
		"+":                 {'+', nil},
		"Enter":             {'\r', nil},
		"enter":             {'\r', nil},
		"Space":             {' ', nil},
		"Control++":         {'+', []chromedp.Modifier{chromedp.ModifierCtrl}},
		"Control+a":         {'a', []chromedp.Modifier{chromedp.ModifierCtrl}},
		"ctrl+Shift+Tab":    {'\t', []chromedp.Modifier{chromedp.ModifierCtrl, chromedp.ModifierShift}},
		"Meta+Alt+ArrowUp":  {keyNames["arrowup"], []chromedp.Modifier{chromedp.ModifierMeta, chromedp.ModifierAlt}},
		"Cmd+Option+Escape": {'\u001b', []chromedp.Modifier{chromedp.ModifierMeta, chromedp.ModifierAlt}},
	} {
		r, mods, err := parseKey(in)
		if err != nil || r != want.r || !slices.Equal(mods, want.mods) {
			t.Errorf("parseKey(%q) = %q, %v, %v; want %q, %v", in, r, mods, err, want.r, want.mods)
		}
	}
}

func TestParseKeyRejectsUnknownNames(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		"Hyper+a": "unknown modifier",
		"a+b":     "unknown modifier",
		"Enterr":  "unknown key",
		"":        "unknown key",
		"Shift+":  "unknown key",
	} {
		if _, _, err := parseKey(in); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("parseKey(%q) error = %v, want %q", in, err, want)
		}
	}
}
