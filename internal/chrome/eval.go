package chrome

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"

	"github.com/joaomdsg/usechrome-mcp/internal/browser"
)

const evalGroup = "usechrome-eval"

// Eval runs a JavaScript expression in a tab and returns its value. The
// DevTools command-line API ($, $$, $x) is in scope.
func (b *Browser) Eval(ctx context.Context, req browser.EvalRequest) (browser.EvalResult, error) {
	var res browser.EvalResult
	err := b.do(ctx, req.Tab, func(tctx context.Context) error {
		// By handle, not by value: by value drops the array subtype and
		// fails on symbols, functions and cyclic objects.
		obj, err := chromedp.Run(tctx, chromedp.Evaluate[*runtime.RemoteObject](req.Expression,
			chromedp.EvalAwaitPromise, chromedp.EvalWithCommandLineAPI, chromedp.EvalObjectGroup(evalGroup)))
		if err != nil {
			return err
		}
		// A failed release only leaves the objects alive until the page goes.
		defer func() {
			_, _ = chromedp.Call(tctx, runtime.ReleaseObjectGroup, runtime.ReleaseObjectGroupParams{ObjectGroup: evalGroup})
		}()
		res = evalResult(tctx, obj)
		return nil
	})
	return res, err
}

func evalResult(ctx context.Context, o *runtime.RemoteObject) browser.EvalResult {
	switch {
	case o.Type == runtime.RemoteObjectTypeUndefined:
		return browser.EvalResult{Type: "undefined"}
	case o.Subtype == runtime.RemoteObjectSubtypeNull:
		return browser.EvalResult{Type: "null", Value: json.RawMessage("null")}
	case o.UnserializableValue != "":
		return browser.EvalResult{Type: string(o.Type), Value: jsonString(string(o.UnserializableValue))}
	case o.ObjectID == "":
		return browser.EvalResult{Type: string(o.Type), Value: json.RawMessage(o.Value)}
	case o.Type == runtime.RemoteObjectTypeObject && (o.Subtype == "" || o.Subtype == runtime.RemoteObjectSubtypeArray):
		return plainObject(ctx, o)
	default:
		typ := string(o.Type)
		if o.Subtype != "" {
			typ = string(o.Subtype)
		}
		return browser.EvalResult{Type: typ, Value: jsonString(o.Description)}
	}
}

// plainObject returns an object or array as JSON, or its description when
// JSON cannot hold it, as with cycles.
func plainObject(ctx context.Context, o *runtime.RemoteObject) browser.EvalResult {
	typ := "object"
	if o.Subtype == runtime.RemoteObjectSubtypeArray {
		typ = "array"
	}
	res, err := chromedp.Call(ctx, runtime.CallFunctionOn, runtime.CallFunctionOnParams{
		FunctionDeclaration: "function() { return this }",
		ObjectID:            o.ObjectID,
		ReturnByValue:       new(true),
	})
	if err == nil && res.ExceptionDetails != nil {
		err = errors.New(res.ExceptionDetails.Text)
	}
	if err != nil {
		return browser.EvalResult{Type: typ, Value: jsonString(o.Description)}
	}
	return browser.EvalResult{Type: typ, Value: json.RawMessage(res.Result.Value)}
}

func jsonString(s string) json.RawMessage {
	b, _ := json.Marshal(s) // a string always marshals
	return b
}
