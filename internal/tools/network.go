package tools

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/joaomdsg/usechrome-mcp/internal/browser"
)

// maxListedURLChars keeps a line of the list short: inline data URLs run to
// thousands of characters. The detail of a request has the whole URL.
const maxListedURLChars = 200

const requestColumns = "id method status type mime size time url\n"

var errFilterAndRequest = errors.New("pass filter or request, not both")

// Networker reads a tab's network log.
type Networker interface {
	Network(ctx context.Context, req browser.NetworkRequest) ([]browser.Request, error)
	RequestDetail(ctx context.Context, req browser.RequestDetailRequest) (browser.RequestDetail, error)
}

// NetworkInput is the arguments of the network tool.
type NetworkInput struct {
	Filter  string `json:"filter,omitempty" jsonschema:"show only the requests whose line contains this text, ignoring case: part of a URL, a type such as fetch, a status such as 404, or failed"`
	Request string `json:"request,omitempty" jsonschema:"id of a request in the list, such as n3, to see its headers and bodies"`
	Target
}

// AddNetwork registers the network tool, which drives n.
func AddNetwork(s *mcp.Server, n Networker) {
	tool := &mcp.Tool{
		Name: "network",
		Description: "List the network requests of a tab's current page, oldest first, as the DevTools Network panel shows them: " +
			"a line per request with its id, method, status, type, MIME type, bytes transferred, time and URL, and why it failed. " +
			"A redirect is a line per hop. Pass request with an id to see its headers and bodies. " +
			"Requests are recorded from when this server opened the tab, which keeps its last 1000.",
		InputSchema: inputSchema[NetworkInput](),
	}
	mcp.AddTool(s, tool, func(ctx context.Context, _ *mcp.CallToolRequest, args NetworkInput) (*mcp.CallToolResult, any, error) {
		if args.Request != "" {
			return requestDetail(ctx, n, args)
		}
		reqs, err := n.Network(ctx, browser.NetworkRequest{Tab: args.Tab})
		if err != nil {
			return nil, nil, err
		}
		return textResult(renderRequests(reqs, args.Filter, maxTextChars)), nil, nil
	})
}

func requestDetail(ctx context.Context, n Networker, args NetworkInput) (*mcp.CallToolResult, any, error) {
	if args.Filter != "" {
		return nil, nil, errFilterAndRequest
	}
	d, err := n.RequestDetail(ctx, browser.RequestDetailRequest{Tab: args.Tab, ID: args.Request})
	if err != nil {
		return nil, nil, err
	}
	return textResult(renderDetail(d, maxTextChars)), nil, nil
}

// renderRequests writes a line per request that matches filter, and cuts
// the oldest past limit characters.
func renderRequests(reqs []browser.Request, filter string, limit int) string {
	var lines []string
	for _, r := range reqs {
		line := requestLine(r)
		if strings.Contains(strings.ToLower(line), strings.ToLower(filter)) {
			lines = append(lines, line)
		}
	}
	switch {
	case len(lines) == 0 && filter == "":
		return "no requests on this page\n"
	case len(lines) == 0:
		return fmt.Sprintf("no requests on this page match %q\n", filter)
	}
	kept, cut := newest(lines, limit-len(requestColumns))
	var b strings.Builder
	if cut > 0 {
		fmt.Fprintf(&b, "... %d earlier requests cut; pass filter to see fewer\n", cut)
	}
	b.WriteString(requestColumns)
	for _, line := range kept {
		b.WriteString(line)
	}
	return b.String()
}

func requestLine(r browser.Request) string {
	line := strings.Join([]string{
		r.ID, r.Method, status(r), cmp.Or(r.Type, "-"), cmp.Or(r.MimeType, "-"), size(r), elapsed(r),
		cutText(r.URL, maxListedURLChars),
	}, " ")
	if r.Error != "" {
		line += " " + r.Error
	}
	return line + "\n"
}

func status(r browser.Request) string {
	switch {
	case r.Error != "":
		return "failed"
	case r.Status == 0:
		return "pending"
	}
	return strconv.Itoa(r.Status)
}

func size(r browser.Request) string {
	switch {
	case !r.Done:
		return "-"
	case r.Cache != "":
		return "(" + r.Cache + ")"
	}
	return byteSize(r.Size)
}

// byteSize uses DevTools' units, where a kB is 1000 bytes.
func byteSize(n int) string {
	switch {
	case n < 1000:
		return fmt.Sprintf("%dB", n)
	case n < 1000*1000:
		return fmt.Sprintf("%.1fkB", float64(n)/1000)
	}
	return fmt.Sprintf("%.1fMB", float64(n)/1000/1000)
}

func elapsed(r browser.Request) string {
	if !r.Done {
		return "-"
	}
	return fmt.Sprintf("%dms", r.Duration.Milliseconds())
}

// renderDetail writes a request with its headers and bodies. The bodies
// share what limit leaves, the request body at most half of it, each with
// room for its title and cut notice. Headers past limit are cut too.
func renderDetail(d browser.RequestDetail, limit int) string {
	var head strings.Builder
	fmt.Fprintf(&head, "%s %s\nstatus: %s\n", d.Method, cutText(d.URL, maxMessageChars), strings.TrimSpace(status(d.Request)+" "+d.StatusText))
	fmt.Fprintf(&head, "type: %s\nmime: %s\nsize: %s\ntime: %s\n", cmp.Or(d.Type, "-"), cmp.Or(d.MimeType, "-"), size(d.Request), elapsed(d.Request))
	if d.Error != "" {
		fmt.Fprintf(&head, "error: %s\n", d.Error)
	}
	sent := headerBlock("request headers", d.RequestHeaders)
	got := headerBlock("response headers", d.ResponseHeaders)
	room := limit - head.Len() - len(sent) - len(got) - 3*noticeRoom
	var sentBody string
	if d.HasPostData {
		sentBody = bodyBlock("request body", d.RequestBody, room/2)
	}
	gotBody := bodyBlock("response body", d.ResponseBody, room-len(sentBody))
	return cutText(head.String()+sent+sentBody+got+gotBody, limit-noticeRoom)
}

func headerBlock(title string, hs []browser.Header) string {
	if len(hs) == 0 {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "\n%s:\n", title)
	for _, h := range hs {
		fmt.Fprintf(&b, "  %s: %s\n", h.Name, h.Value)
	}
	return b.String()
}

func bodyBlock(title string, body browser.Body, room int) string {
	switch {
	case body.Missing != "":
		return fmt.Sprintf("\n%s: none, %s\n", title, body.Missing)
	case body.Binary:
		return fmt.Sprintf("\n%s: %s of binary data\n", title, byteSize(body.Size))
	}
	return fmt.Sprintf("\n%s (%s):\n%s\n", title, byteSize(body.Size), cutText(body.Text, max(room, 0)))
}
