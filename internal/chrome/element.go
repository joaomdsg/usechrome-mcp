package chrome

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"

	"github.com/chromedp/cdproto"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

// elementGroup holds the handles of picked elements so that one call
// releases them all.
const elementGroup = "usechrome-element"

var refSyntax = regexp.MustCompile(`^e([1-9][0-9]*)$`)

// element returns a handle to the element that ref, from a snapshot, or the
// CSS selector sel picks. Release it with releaseElements.
func element(ctx context.Context, ref, sel string) (runtime.RemoteObjectID, error) {
	if ref != "" {
		return refElement(ctx, ref)
	}
	obj, err := chromedp.Run(ctx, chromedp.Evaluate[*runtime.RemoteObject](
		fmt.Sprintf("document.querySelector(%s)", jsonString(sel)), chromedp.EvalObjectGroup(elementGroup)))
	if err != nil {
		return "", err
	}
	if obj.ObjectID == "" {
		return "", fmt.Errorf("no element matches %q", sel)
	}
	return obj.ObjectID, nil
}

func refElement(ctx context.Context, ref string) (runtime.RemoteObjectID, error) {
	var id int64
	m := refSyntax.FindStringSubmatch(ref)
	if m != nil {
		if n, err := strconv.ParseInt(m[1], 10, 64); err == nil {
			id = n
		}
	}
	if id == 0 {
		return "", fmt.Errorf("%q is not a ref from the snapshot tool, such as e12", ref)
	}
	res, err := chromedp.Call(ctx, dom.ResolveNode, dom.ResolveNodeParams{BackendNodeID: cdp.BackendNodeID(id), ObjectGroup: elementGroup})
	if cerr := (*cdproto.Error)(nil); errors.As(err, &cerr) {
		return "", fmt.Errorf("ref %s is not in the page; take a new snapshot", ref)
	}
	if err != nil {
		return "", err
	}
	return res.Object.ObjectID, nil
}

func releaseElements(ctx context.Context) {
	_, _ = chromedp.Call(ctx, runtime.ReleaseObjectGroup, runtime.ReleaseObjectGroupParams{ObjectGroup: elementGroup})
}

// callOn runs the JavaScript function fn with this set to el and decodes
// what it returns.
func callOn[T any](ctx context.Context, el runtime.RemoteObjectID, fn string, args ...any) (T, error) {
	var out T
	params := runtime.CallFunctionOnParams{FunctionDeclaration: fn, ObjectID: el, ReturnByValue: new(true)}
	for _, a := range args {
		b, err := json.Marshal(a)
		if err != nil {
			return out, err
		}
		params.Arguments = append(params.Arguments, &runtime.CallArgument{Value: b})
	}
	res, err := chromedp.Call(ctx, runtime.CallFunctionOn, params)
	if err != nil {
		return out, err
	}
	if d := res.ExceptionDetails; d != nil {
		if d.Exception != nil && d.Exception.Description != "" {
			return out, errors.New(d.Exception.Description)
		}
		return out, errors.New(d.Text)
	}
	if len(res.Result.Value) > 0 {
		err = json.Unmarshal(res.Result.Value, &out)
	}
	return out, err
}
