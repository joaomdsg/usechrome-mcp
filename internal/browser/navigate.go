package browser

// NavAction names a way to move a tab through its history.
type NavAction string

// The navigation actions a tab supports.
const (
	Goto    NavAction = "goto"
	Back    NavAction = "back"
	Forward NavAction = "forward"
	Reload  NavAction = "reload"
)

// NavRequest asks a tab to navigate. An empty Tab means the current tab.
type NavRequest struct {
	Tab    string
	Action NavAction
	URL    string
}

// Page describes the document a tab shows after a navigation. Status is the
// HTTP status of the document's response, or 0 when the browser reports
// none, such as for a file: URL.
type Page struct {
	URL    string
	Status int
	Title  string
}
