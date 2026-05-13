package tui

import "github.com/loglens/loglens/internal/correlate"

// detector is the request-id detector consulted by the detail pane when a row
// is selected. main.go calls SetDetector once at startup after parsing
// --correlate. Defined as a package-level value rather than a Model field so
// this file does not collide with the in-flight row-list scaffolding in
// model.go; the row-integration child issue will fold it into Model when it
// touches the rest of the layout.
var detector = correlate.Default()

// SetDetector installs the request-id detector for the running program.
// Subsequent calls to Detector return the new value.
func SetDetector(d correlate.Detector) {
	if len(d.Keys()) == 0 {
		return
	}
	detector = d
}

// Detector returns the active request-id detector. Always returns a usable
// value (defaults are installed if SetDetector was never called).
func Detector() correlate.Detector { return detector }
