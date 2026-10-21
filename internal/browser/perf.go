package browser

import "time"

// PerfRequest asks how a tab's current page has performed.
type PerfRequest struct {
	Tab string
}

// Perf is what the browser measured on a tab's current page since it began
// to load. Times count from then; a zero time means the page has not got
// there yet. Interactions holds only the interactions of 104 ms or more,
// the ones the browser keeps; InteractionCount counts them all. Frames are
// the animation frames that took over 50 ms. Elements counts the page's
// elements; the heap is that of the tab's renderer process.
type Perf struct {
	URL              string
	TTFB             time.Duration
	FCP              time.Duration
	LCP              time.Duration
	LCPElement       string
	DOMContentLoaded time.Duration
	Load             time.Duration
	Shifts           []Shift
	InteractionCount int
	Interactions     []InteractionTiming
	Frames           []Frame
	Elements         int
	HeapUsed         int
	HeapTotal        int
}

// Shift is a layout shift and the elements it moved. Input is whether the
// user had just interacted, which excuses the shift.
type Shift struct {
	Start time.Duration
	Value float64
	Input bool
	Moved []string
}

// InteractionTiming is a click, tap or key press, told by the event whose
// handlers ran longest and the element that got it. Its duration runs to
// the next frame and splits into the wait for the handlers, the handlers,
// and the rendering after. The browser rounds the duration to 8 ms, so it
// can come out a little under the handlers' time.
type InteractionTiming struct {
	Event        string
	Target       string
	Start        time.Duration
	Duration     time.Duration
	InputDelay   time.Duration
	Processing   time.Duration
	Presentation time.Duration
}

// Frame is a long animation frame: Blocking is the part of its long tasks
// past 50 ms each, which held up input.
type Frame struct {
	Start    time.Duration
	Duration time.Duration
	Blocking time.Duration
	Scripts  []Script
}

// Script is a script that ran in a long frame. Invoker is what ran it, such
// as "BUTTON#buy.onclick"; Kind is how, such as "event-listener" or
// "classic-script"; Char is where its function starts in its script: the
// file at URL, or the text of an inline script element.
type Script struct {
	Invoker  string
	Kind     string
	Function string
	URL      string
	Char     int
	Duration time.Duration
}
