package chrome

import (
	"context"
	"fmt"
	"slices"
	"testing"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

func TestConsoleMessageCutsExceptionStackAtTenFrames(t *testing.T) {
	m := chromedp.ConsoleMessage{Type: chromedp.ConsoleException, Text: "Error: boom", Stack: &runtime.StackTrace{}}
	var want []string
	for i := range 11 {
		m.Stack.CallFrames = append(m.Stack.CallFrames, &runtime.CallFrame{FunctionName: fmt.Sprintf("f%d", i), URL: "http://x/a.js", LineNumber: int64(i)})
		if i < 10 {
			want = append(want, fmt.Sprintf("f%d (http://x/a.js:%d:1)", i, i+1))
		}
	}
	if got := consoleMessage(context.Background(), m).Stack; !slices.Equal(got, want) {
		t.Errorf("stack:\n got %q\nwant %q", got, want)
	}
}

func TestFrameNamesFunctionOnlyWhenItHasOne(t *testing.T) {
	for _, c := range []struct {
		f    runtime.CallFrame
		want string
	}{
		{runtime.CallFrame{FunctionName: "go", URL: "http://x/a.js", LineNumber: 2, ColumnNumber: 4}, "go (http://x/a.js:3:5)"},
		{runtime.CallFrame{URL: "http://x/a.js", LineNumber: 2, ColumnNumber: 4}, "http://x/a.js:3:5"},
	} {
		if got := frame(&c.f); got != c.want {
			t.Errorf("frame(%+v) = %q; want %q", c.f, got, c.want)
		}
	}
}

func TestIsPlainObjectTakesOnlyObjectsAndArraysWithAnID(t *testing.T) {
	for _, c := range []struct {
		o    runtime.RemoteObject
		want bool
	}{
		{runtime.RemoteObject{Type: runtime.RemoteObjectTypeObject, ObjectID: "1"}, true},
		{runtime.RemoteObject{Type: runtime.RemoteObjectTypeObject, Subtype: runtime.RemoteObjectSubtypeArray, ObjectID: "1"}, true},
		{runtime.RemoteObject{Type: runtime.RemoteObjectTypeObject, Subtype: runtime.RemoteObjectSubtypeError, ObjectID: "1"}, false},
		{runtime.RemoteObject{Type: runtime.RemoteObjectTypeObject}, false},
		{runtime.RemoteObject{Type: runtime.RemoteObjectTypeObject, Subtype: runtime.RemoteObjectSubtypeArray}, false},
		{runtime.RemoteObject{Type: runtime.RemoteObjectTypeFunction, ObjectID: "1"}, false},
	} {
		if got := isPlainObject(&c.o); got != c.want {
			t.Errorf("isPlainObject(%+v) = %v; want %v", c.o, got, c.want)
		}
	}
}

func TestIsFormatTakesOnlyStringsWithAPercent(t *testing.T) {
	for _, c := range []struct {
		o    runtime.RemoteObject
		want bool
	}{
		{runtime.RemoteObject{Type: runtime.RemoteObjectTypeString, Value: []byte(`"%o"`)}, true},
		{runtime.RemoteObject{Type: runtime.RemoteObjectTypeString, Value: []byte(`"o"`)}, false},
		{runtime.RemoteObject{Type: runtime.RemoteObjectTypeObject, Value: []byte(`"%o"`)}, false},
	} {
		if got := isFormat(&c.o); got != c.want {
			t.Errorf("isFormat(%+v) = %v; want %v", c.o, got, c.want)
		}
	}
}
