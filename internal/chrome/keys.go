package chrome

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/kb"
)

var modifierNames = map[string]chromedp.Modifier{
	"control": chromedp.ModifierCtrl,
	"ctrl":    chromedp.ModifierCtrl,
	"alt":     chromedp.ModifierAlt,
	"option":  chromedp.ModifierAlt,
	"shift":   chromedp.ModifierShift,
	"meta":    chromedp.ModifierMeta,
	"command": chromedp.ModifierMeta,
	"cmd":     chromedp.ModifierMeta,
}

// keyNames maps lower-cased DOM key names, such as "enter" and "arrowdown",
// to the runes kb encodes them from.
var keyNames = func() map[string]rune {
	names := map[string]rune{"space": ' '}
	for r, k := range kb.Keys {
		if utf8.RuneCountInString(k.Key) > 1 {
			names[strings.ToLower(k.Key)] = r
		}
	}
	return names
}()

// parseKey reads a key such as "a", "Enter" or "Control+Shift+Tab".
func parseKey(s string) (rune, []chromedp.Modifier, error) {
	var mods []chromedp.Modifier
	name := s
	// A '+' that ends the string, or is all of it, is the key itself.
	for i := strings.Index(name, "+"); i > 0 && i < len(name)-1; i = strings.Index(name, "+") {
		m, ok := modifierNames[strings.ToLower(name[:i])]
		if !ok {
			return 0, nil, fmt.Errorf("unknown modifier %q in %q; use Control, Alt, Shift or Meta", name[:i], s)
		}
		mods = append(mods, m)
		name = name[i+1:]
	}
	if utf8.RuneCountInString(name) == 1 {
		r, _ := utf8.DecodeRuneInString(name)
		return r, mods, nil
	}
	if r, ok := keyNames[strings.ToLower(name)]; ok {
		return r, mods, nil
	}
	return 0, nil, fmt.Errorf("unknown key %q; use a character or a key name such as Enter, Tab, Escape, ArrowDown or F5", name)
}
