package dsl

import (
	"strings"
	"time"
)

// Row is one line of the network list.
type Row struct {
	ID, Method, Status, Type, Mime, Size, Time, URL string
}

// Requests lists the current page's requests whose line contains filter,
// waiting up to 5 seconds until there are want of them and none is loading.
// The favicon, which the browser asks for after the load, is left out.
func (a *Agent) Requests(filter string, want int) []Row {
	a.t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		rows := a.rows(filter)
		if (len(rows) >= want && finished(rows)) || time.Now().After(deadline) {
			return rows
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// Request reads the detail of the request with id.
func (a *Agent) Request(id string) string {
	a.t.Helper()
	return a.call("network", map[string]any{"request": id}, nil).Text
}

// TryRequest is Request for a call expected to fail; it returns the failure.
func (a *Agent) TryRequest(id string) error {
	a.t.Helper()
	return a.try("network", map[string]any{"request": id})
}

func (a *Agent) rows(filter string) []Row {
	a.t.Helper()
	text := a.call("network", map[string]any{"filter": filter}, nil).Text
	var rows []Row
	for line := range strings.Lines(text) {
		f := strings.Fields(line)
		if len(f) < 8 || f[0] == "id" || strings.HasSuffix(f[7], "/favicon.ico") {
			continue
		}
		rows = append(rows, Row{ID: f[0], Method: f[1], Status: f[2], Type: f[3], Mime: f[4], Size: f[5], Time: f[6], URL: f[7]})
	}
	return rows
}

func finished(rows []Row) bool {
	for _, r := range rows {
		if r.Time == "-" {
			return false
		}
	}
	return true
}
