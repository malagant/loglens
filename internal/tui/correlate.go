package tui

// The request-id detector formerly held here as a package-level variable has
// been folded into Model.detector (SPA-33). Use Model.WithDetector to install
// a custom detector; correlate.Default() is pre-installed by New().
