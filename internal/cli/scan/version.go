package scan

// Version is set from cmd/krit/main.go at startup. Goreleaser
// populates main.version via -X linker flags; main.go propagates that
// value into scan.Version before the scan path runs.
var Version = "dev"
