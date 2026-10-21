package chrome

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/log"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"

	"github.com/joaomdsg/usechrome-mcp/internal/browser"
)

// The browser sends the stored messages before it answers the enable
// commands, so after the answers they only cross chromedp's queues; a gap
// of replayIdle means they are all in. replayMax bounds a page that keeps
// logging while it is read.
const (
	replayIdle = 100 * time.Millisecond
	replayMax  = time.Second
)

// maxStackFrames matches V8's default Error.stackTraceLimit.
const maxStackFrames = 10

// Console reads the console of a tab's current page as DevTools keeps it.
// Enabling the Runtime and Log domains replays the page's stored messages,
// so a second session to the tab enables them afresh, leaving the session
// that the other tools use alone.
func (b *Browser) Console(ctx context.Context, req browser.ConsoleRequest) ([]browser.ConsoleMessage, error) {
	var out []browser.ConsoleMessage
	err := b.do(ctx, req.Tab, func(tctx context.Context) error {
		id := chromedp.FromContext(tctx).Target.TargetID
		sctx, cancel := chromedp.NewContext(tctx, chromedp.WithTargetID(id), chromedp.WithDetachOnCancel())
		defer cancel()
		if err := chromedp.Do(sctx); err != nil {
			return err
		}
		stored, err := replayConsole(sctx)
		if err != nil {
			return err
		}
		out = make([]browser.ConsoleMessage, 0, len(stored))
		for _, m := range stored {
			out = append(out, consoleMessage(sctx, m))
		}
		return nil
	})
	return out, err
}

func replayConsole(ctx context.Context) ([]chromedp.ConsoleMessage, error) {
	rctx, cancel := context.WithTimeout(ctx, replayMax)
	defer cancel()
	messages := chromedp.Console(rctx)
	for _, c := range []cdp.Command[cdp.Empty, cdp.Empty]{runtime.Disable, runtime.Enable, log.Disable, log.Enable} {
		if _, err := chromedp.Call(ctx, c, cdp.Empty{}); err != nil {
			return nil, err
		}
	}
	idle := time.AfterFunc(replayIdle, cancel)
	defer idle.Stop()
	var stored []chromedp.ConsoleMessage
	for m, err := range messages {
		if err != nil {
			break
		}
		stored = append(stored, m)
		idle.Reset(replayIdle)
	}
	// The replay sends the Runtime domain's messages before the Log domain's.
	slices.SortStableFunc(stored, func(a, b chromedp.ConsoleMessage) int { return a.Time.Compare(b.Time) })
	return stored, nil
}

func consoleMessage(ctx context.Context, m chromedp.ConsoleMessage) browser.ConsoleMessage {
	out := browser.ConsoleMessage{Type: string(m.Type), Text: consoleText(ctx, m), Source: m.URL}
	// A browser message's line is absent rather than 0, so only calls and
	// exceptions get one.
	if m.Source == "" && m.URL != "" {
		out.Source = fmt.Sprintf("%s:%d:%d", m.URL, m.Line+1, m.Column+1)
	}
	if m.IsException() && m.Stack != nil {
		for _, f := range m.Stack.CallFrames[:min(len(m.Stack.CallFrames), maxStackFrames)] {
			out.Stack = append(out.Stack, frame(f))
		}
	}
	return out
}

func frame(f *runtime.CallFrame) string {
	at := fmt.Sprintf("%s:%d:%d", f.URL, f.LineNumber+1, f.ColumnNumber+1)
	if f.FunctionName == "" {
		return at
	}
	return f.FunctionName + " (" + at + ")"
}

// consoleText writes objects and arrays as JSON. Replayed messages come
// without the previews chromedp writes them from, so its text says only
// "Object". A format string such as "%o" keeps chromedp's text, since its
// arguments go into the format.
func consoleText(ctx context.Context, m chromedp.ConsoleMessage) string {
	if !slices.ContainsFunc(m.Args, isPlainObject) || isFormat(m.Args[0]) {
		return m.Text
	}
	parts := make([]string, 0, len(m.Args))
	for _, a := range m.Args {
		parts = append(parts, argText(ctx, a))
	}
	return strings.Join(parts, " ")
}

// argText is a value as JSON, but a string, or a description that JSON
// cannot hold, as itself.
func argText(ctx context.Context, a *runtime.RemoteObject) string {
	v := evalResult(ctx, a)
	if v.Value == nil {
		return v.Type
	}
	var s string
	if json.Unmarshal(v.Value, &s) == nil {
		return s
	}
	return string(v.Value)
}

func isPlainObject(o *runtime.RemoteObject) bool {
	return o.Type == runtime.RemoteObjectTypeObject && o.ObjectID != "" &&
		(o.Subtype == "" || o.Subtype == runtime.RemoteObjectSubtypeArray)
}

func isFormat(o *runtime.RemoteObject) bool {
	return o.Type == runtime.RemoteObjectTypeString && strings.Contains(string(o.Value), "%")
}
