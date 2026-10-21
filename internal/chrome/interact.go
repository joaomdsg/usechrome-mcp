package chrome

import (
	"context"
	"errors"
	"fmt"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"

	"github.com/joaomdsg/usechrome-mcp/internal/browser"
)

// Interact acts on an element as a person would and reports the page once
// any navigation the action started has loaded.
func (b *Browser) Interact(ctx context.Context, req browser.InteractRequest) (browser.Interaction, error) {
	var out browser.Interaction
	err := b.do(ctx, req.Tab, func(tctx context.Context) error {
		w, err := watchPage(tctx)
		if err != nil {
			return err
		}
		defer w.stop()
		if err := act(tctx, req); err != nil {
			return err
		}
		if err := w.settle(tctx); err != nil {
			return err
		}
		d, err := readDocument(tctx)
		out = browser.Interaction{URL: d.URL, Title: d.Title, Navigated: w.navigated(), Dialogs: w.opened()}
		return err
	})
	return out, err
}

func act(ctx context.Context, req browser.InteractRequest) error {
	defer releaseElements(ctx)
	var el runtime.RemoteObjectID
	if req.Ref != "" || req.Selector != "" {
		var err error
		if el, err = element(ctx, req.Ref, req.Selector); err != nil {
			return err
		}
	}
	var problem string
	var err error
	switch req.Action {
	case browser.Click, browser.Hover:
		problem, err = point(ctx, el, req.Action)
	case browser.Type:
		problem, err = typeText(ctx, el, req.Text)
	case browser.Press:
		problem, err = press(ctx, el, req.Text)
	case browser.Select:
		problem, err = selectOption(ctx, el, req.Text)
	default:
		return fmt.Errorf("unknown action %q", req.Action)
	}
	if problem != "" {
		return fmt.Errorf("%s %s", described(req), problem)
	}
	return err
}

var errNoElement = errors.New("needs a ref or a selector")

func described(req browser.InteractRequest) string {
	if req.Ref != "" {
		return "element " + req.Ref
	}
	return fmt.Sprintf("%q", req.Selector)
}

// pointJS scrolls the element into view and finds the centre of its visible
// part. Clicking a label's covering part still reaches its control, as for
// a styled checkbox.
const pointJS = `function() {
	if (!this.isConnected) return {problem: "is no longer in the page; take a new snapshot"};
	this.scrollIntoViewIfNeeded(true);
	const r = [...this.getClientRects()].find(r => r.width > 0 && r.height > 0);
	if (!r) return {problem: "is not visible"};
	const left = Math.max(r.left, 0), right = Math.min(r.right, innerWidth);
	const top = Math.max(r.top, 0), bottom = Math.min(r.bottom, innerHeight);
	if (right <= left || bottom <= top) return {problem: "is outside the viewport"};
	const x = (left + right) / 2, y = (top + bottom) / 2;
	const hit = this.getRootNode().elementFromPoint(x, y);
	if (hit && hit !== this && !this.contains(hit) && hit.closest("label")?.control !== this) {
		const name = hit.localName + (hit.id ? "#" + hit.id : "") + [...hit.classList].map(c => "." + c).join("");
		return {problem: "is covered by " + name};
	}
	return {x, y, disabled: this.matches(":disabled")};
}`

type spot struct {
	X, Y     float64
	Disabled bool
	Problem  string
}

// point moves the mouse to the element and, for a click, presses it.
func point(ctx context.Context, el runtime.RemoteObjectID, g browser.Gesture) (string, error) {
	if el == "" {
		return "", fmt.Errorf("%s %w", g, errNoElement)
	}
	s, err := callOn[spot](ctx, el, pointJS)
	if err != nil || s.Problem != "" {
		return s.Problem, err
	}
	if g == browser.Click && s.Disabled {
		return "is disabled", nil
	}
	actions := []chromedp.Action[chromedp.Void]{chromedp.MouseEvent(chromedp.MouseMoved, s.X, s.Y)}
	if g == browser.Click {
		actions = append(actions, chromedp.MouseClickXY(s.X, s.Y))
	}
	return "", chromedp.Do(ctx, actions...)
}

// focusJS focuses the element and, for typing, selects its content so that
// typing replaces it.
const focusJS = `function(selectAll) {
	if (!this.isConnected) return "is no longer in the page; take a new snapshot";
	if (this.matches(":disabled")) return "is disabled";
	if (selectAll) {
		const field = this.matches("input, textarea");
		if (!field && !this.isContentEditable) return "is not a text field";
		if (this.readOnly) return "is read-only";
	}
	this.scrollIntoViewIfNeeded(true);
	this.focus();
	if (!this.contains(this.getRootNode().activeElement)) return "cannot take focus";
	if (selectAll) {
		if (this.matches("input, textarea")) this.select();
		else getSelection().selectAllChildren(this);
	}
	return "";
}`

// typeText types text with key presses, in place of the element's content
// when there is an element, or at the focus when there is none.
func typeText(ctx context.Context, el runtime.RemoteObjectID, text string) (string, error) {
	if el != "" {
		if problem, err := callOn[string](ctx, el, focusJS, true); err != nil || problem != "" {
			return problem, err
		}
	}
	if text == "" {
		if el == "" {
			return "", errors.New("type needs text, or an element to clear")
		}
		return "", chromedp.Do(ctx, chromedp.KeyEvent("\b"))
	}
	return "", chromedp.Do(ctx, chromedp.KeyEvent(text))
}

// press presses a key, on the element when there is one.
func press(ctx context.Context, el runtime.RemoteObjectID, key string) (string, error) {
	r, mods, err := parseKey(key)
	if err != nil {
		return "", err
	}
	if el != "" {
		if problem, err := callOn[string](ctx, el, focusJS, false); err != nil || problem != "" {
			return problem, err
		}
	}
	return "", chromedp.Do(ctx, chromedp.KeyEvent(string(r), chromedp.KeyModifiers(mods...)))
}

// selectJS picks an option by value, or else by label, and fires the events
// a person's pick fires.
const selectJS = `function(want) {
	if (!this.isConnected) return "is no longer in the page; take a new snapshot";
	if (this.localName !== "select") return "is not a <select> element";
	if (this.disabled) return "is disabled";
	const options = [...this.options];
	const o = options.find(o => o.value === want) ?? options.find(o => o.label.trim() === want);
	if (!o) return "has no option " + JSON.stringify(want) + "; its options are " + options.map(o => JSON.stringify(o.label.trim())).join(", ");
	if (o.disabled) return "has option " + JSON.stringify(want) + " disabled";
	this.focus();
	o.selected = true;
	this.dispatchEvent(new Event("input", {bubbles: true}));
	this.dispatchEvent(new Event("change", {bubbles: true}));
	return "";
}`

func selectOption(ctx context.Context, el runtime.RemoteObjectID, option string) (string, error) {
	if el == "" {
		return "", fmt.Errorf("select %w", errNoElement)
	}
	return callOn[string](ctx, el, selectJS, option)
}
