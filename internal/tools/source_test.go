package tools_test

import (
	"context"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/joaomdsg/usechrome-mcp/internal/browser"
	"github.com/joaomdsg/usechrome-mcp/internal/tools"
)

type fakeSourcer struct {
	got []browser.SourceRequest
	src browser.Source
	err error
}

func (f *fakeSourcer) Source(_ context.Context, req browser.SourceRequest) (browser.Source, error) {
	f.got = append(f.got, req)
	return f.src, f.err
}

func sourceSession(t *testing.T, s tools.Sourcer) *mcp.ClientSession {
	t.Helper()
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	tools.AddSource(srv, s)
	return connect(t, srv)
}

func textSource(url, text string) browser.Source {
	return browser.Source{URL: url, MimeType: "text/javascript", Body: browser.Body{Text: text, Size: len(text)}}
}

func TestSourceWritesTheSourceAfterWhatItIs(t *testing.T) {
	t.Parallel()
	s := &fakeSourcer{src: textSource("http://x/app.js", "let app\n")}
	res := call(t, sourceSession(t, s), "source", map[string]any{"url": "http://x/app.js", "tab": "t2"})
	if got, want := errorText(res), "http://x/app.js (text/javascript, 8B):\nlet app\n"; res.IsError || got != want {
		t.Errorf("source = %q, want %q", got, want)
	}
	if want := (browser.SourceRequest{Tab: "t2", URL: "http://x/app.js"}); len(s.got) != 1 || s.got[0] != want {
		t.Errorf("browser called with %+v, want %+v", s.got, want)
	}
}

var readOn = regexp.MustCompile(`\n\.\.\. \d+ more characters; pass offset (\d+) to read on\n$`)

func TestSourceReadsALongSourceInParts(t *testing.T) {
	t.Parallel()
	text := strings.Repeat("const s = \"é\";\n", 7000)
	cs := sourceSession(t, &fakeSourcer{src: textSource("http://x/big.js", text)})
	var read strings.Builder
	offset, parts := 0, 0
	for parts = 1; parts < 10; parts++ {
		part := errorText(call(t, cs, "source", map[string]any{"url": "http://x/big.js", "offset": offset}))
		if len(part) > 40_000 {
			t.Fatalf("part %d is %d chars, want at most 40000", parts, len(part))
		}
		_, body, _ := strings.Cut(part, ":\n")
		next := readOn.FindStringSubmatch(body)
		if next == nil {
			read.WriteString(body)
			break
		}
		read.WriteString(strings.TrimSuffix(body, next[0]))
		offset, _ = strconv.Atoi(next[1])
	}
	if parts != 3 || read.String() != text {
		t.Errorf("read %d parts, %d of %d bytes; want 3 parts that together are the source", parts, read.Len(), len(text))
	}
}

func TestSourceStartsAnOffsetOnAWholeCharacter(t *testing.T) {
	t.Parallel()
	s := &fakeSourcer{src: textSource("http://x/a.js", "éa")}
	got := errorText(call(t, sourceSession(t, s), "source", map[string]any{"offset": 1}))
	if want := "http://x/a.js (text/javascript, 3B) from offset 2:\na"; got != want {
		t.Errorf("source from inside é = %q, want %q", got, want)
	}
}

func TestSourceRejectsAnOffsetPastTheEnd(t *testing.T) {
	t.Parallel()
	for _, offset := range []int{9, -1} {
		res := call(t, sourceSession(t, &fakeSourcer{src: textSource("http://x/a.js", "let a")}), "source", map[string]any{"offset": offset})
		if !res.IsError || !strings.Contains(errorText(res), "offset") {
			t.Errorf("source from offset %d = %q, want an error about the offset", offset, errorText(res))
		}
	}
}

func TestSourceGivesTheSizeOfABinaryFile(t *testing.T) {
	t.Parallel()
	s := &fakeSourcer{src: browser.Source{URL: "http://x/logo.png", MimeType: "image/png", Body: browser.Body{Size: 2048, Binary: true}}}
	if got, want := errorText(call(t, sourceSession(t, s), "source", map[string]any{})), "http://x/logo.png (image/png, 2.0kB) of binary data\n"; got != want {
		t.Errorf("source = %q, want %q", got, want)
	}
}

func TestSourceReportsBrowserError(t *testing.T) {
	t.Parallel()
	res := call(t, sourceSession(t, &fakeSourcer{err: errors.New("the page loaded no resource http://x/nope.js")}), "source", map[string]any{"url": "http://x/nope.js"})
	if !res.IsError || !strings.Contains(errorText(res), "nope.js") {
		t.Errorf("result = %+v, want the browser error", res)
	}
}
