package chrome

import (
	"slices"
	"testing"

	"github.com/chromedp/cdproto/target"
)

func watchClosed() <-chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}

func TestNavigatedReportsCommitOrSameDocumentNavigation(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		committed, sameDoc bool
		want               bool
	}{
		"neither":   {false, false, false},
		"committed": {true, false, true},
		"same doc":  {false, true, true},
		"both":      {true, true, true},
	} {
		w := &pageWatch{committed: make(chan struct{}), sameDoc: make(chan struct{})}
		if tc.committed {
			w.committed = watchClosed()
		}
		if tc.sameDoc {
			w.sameDoc = watchClosed()
		}
		if got := w.navigated(); got != tc.want {
			t.Errorf("%s: navigated() = %v; want %v", name, got, tc.want)
		}
	}
}

func TestOpenedAndPopupsReturnWhatWasRecorded(t *testing.T) {
	t.Parallel()
	w := &pageWatch{
		dialogs: []string{"alert: hi"},
		created: []target.ID{"T1"},
		windows: []string{"https://example.com/"},
	}
	if got := w.opened(); !slices.Equal(got, w.dialogs) {
		t.Errorf("opened() = %v; want %v", got, w.dialogs)
	}
	created, windows := w.popups()
	if !slices.Equal(created, w.created) || !slices.Equal(windows, w.windows) {
		t.Errorf("popups() = %v, %v; want %v, %v", created, windows, w.created, w.windows)
	}
}
