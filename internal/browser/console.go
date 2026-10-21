package browser

// ConsoleRequest asks for the console messages of a tab's current page.
type ConsoleRequest struct {
	Tab string
}

// ConsoleMessage is one entry of the DevTools console. Type is "log",
// "info", "debug", "warning", "error", "exception" or another console call
// such as "assert" or "trace". Source is where it came from: url:line:column
// of the call or the throw, or the URL a browser message is about. Stack
// lists the frames of an exception, innermost first.
type ConsoleMessage struct {
	Type   string
	Text   string
	Source string
	Stack  []string
}
