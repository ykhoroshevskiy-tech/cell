package version

// Version is overridden at build time via -ldflags; falls back to a dev marker.
var Version = "0.0.0+dev"
