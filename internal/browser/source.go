package browser

// SourceRequest asks for the source of a resource of a tab's current page:
// a document, its frames' included, a script, a stylesheet or another file
// it loaded. An empty URL means the page's own document.
type SourceRequest struct {
	Tab string
	URL string
}

// Source is a resource as the server sent it, before any script ran.
type Source struct {
	URL      string
	MimeType string
	Body
}
