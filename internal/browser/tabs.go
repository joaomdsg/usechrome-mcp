package browser

// TabAction names what to do with the browser's tabs before listing them.
type TabAction string

// The tab actions.
const (
	ListTabs  TabAction = "list"
	OpenTab   TabAction = "open"
	SelectTab TabAction = "select"
	CloseTab  TabAction = "close"
)

// TabsRequest asks to list the browser's tabs, after opening, selecting or
// closing one. Tab is the tab to select or close, the current one when
// empty; URL is what a new tab loads, a blank page when empty.
type TabsRequest struct {
	Action TabAction
	Tab    string
	URL    string
}

// Tab is an open tab. Current marks the tab that requests with no tab act
// on.
type Tab struct {
	ID      string
	Title   string
	URL     string
	Current bool
}
