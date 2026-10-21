package browser

import "time"

// NetworkRequest asks for the requests of a tab's current page.
type NetworkRequest struct {
	Tab string
}

// RequestDetailRequest asks for one request, by the ID a Request has.
type RequestDetailRequest struct {
	Tab string
	ID  string
}

// Request is one row of the DevTools Network panel: a request and its
// response, or one hop of a redirect chain. Status is 0 until a response
// arrives, and Error says why a request failed. Done is false while it
// loads. Size is the bytes transferred, headers included, and Cache names
// the cache that served it instead, such as "disk cache".
type Request struct {
	ID              string
	Method          string
	URL             string
	Type            string
	Status          int
	StatusText      string
	MimeType        string
	Size            int
	Cache           string
	Duration        time.Duration
	Done            bool
	Error           string
	RequestHeaders  []Header
	ResponseHeaders []Header
	HasPostData     bool
}

// Header is one HTTP header line.
type Header struct {
	Name  string
	Value string
}

// RequestDetail is a request with its bodies.
type RequestDetail struct {
	Request
	RequestBody  Body
	ResponseBody Body
}

// Body is the body of a request or a response. Missing says why there is
// none, such as for a redirect or a request still loading. Data that is not
// UTF-8 text is Binary, and only its size is kept.
type Body struct {
	Text    string
	Size    int
	Binary  bool
	Missing string
}
