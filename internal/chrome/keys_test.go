package chrome

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

func TestParseKeyStopsAfterTheLastModifier(t *testing.T) {
	done := make(chan rune, 1)
	go func() {
		r, _, _ := parseKey("Control+Shift+a")
		done <- r
	}()
	// A loop that never shrinks name spins forever, so bound it. First in the
	// file: the unbounded parseKey tests below would spin before this ran.
	select {
	case r := <-done:
		if r != 'a' {
			t.Errorf("parseKey(%q) = %q, want %q", "Control+Shift+a", r, 'a')
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("parseKey(%q) did not return", "Control+Shift+a")
	}
}

func TestParseKeyReadsNamesCharactersAndModifiers(t *testing.T) {
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

func TestParseKeyErrorNamesTheWholeUnparsedKey(t *testing.T) {
	for in, want := range map[string]string{
		"a+":             `unknown key "a+"`,
		"+a":             `unknown key "+a"`,
		"++":             `unknown key "++"`,
		"Shift+":         `unknown key "Shift+"`,
		"Control+Shift+": `unknown key "Shift+"`,
		"Hyper+a":        `unknown modifier "Hyper" in "Hyper+a"`,
	} {
		r, mods, err := parseKey(in)
		if err == nil || !strings.HasPrefix(err.Error(), want) {
			t.Errorf("parseKey(%q) error = %v, want prefix %q", in, err, want)
		}
		if r != 0 || mods != nil {
			t.Errorf("parseKey(%q) = %q, %v on error; want 0, nil", in, r, mods)
		}
	}
}

func TestParseKeyKnowsTwoRuneAndLongerKeyNames(t *testing.T) {
	for _, name := range []string{"F5", "Enter", "ArrowDown"} {
		if _, _, err := parseKey(name); err != nil {
			t.Errorf("parseKey(%q) error = %v, want none", name, err)
		}
	}
}
