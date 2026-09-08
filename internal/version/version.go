// Package version exposes build metadata injected at link time.
//
// The release pipeline overrides these variables with -ldflags, e.g.
//
//	go build -ldflags "-X github.com/ValentinoTriadi/ci-cd-example/internal/version.Version=v1.2.3"
package version

import "runtime"

// Values injected via -ldflags at build time. The defaults are what you get
// from a plain `go build` or `go run`, which is how you can tell a local
// binary from a pipeline-built one.
var (
	Version   = "dev"
	Commit    = "none"
	BuildDate = "unknown"
)

// Info is the payload served by GET /version.
type Info struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildDate string `json:"buildDate"`
	GoVersion string `json:"goVersion"`
	Platform  string `json:"platform"`
}

// Get returns the current build metadata.
func Get() Info {
	return Info{
		Version:   Version,
		Commit:    Commit,
		BuildDate: BuildDate,
		GoVersion: runtime.Version(),
		Platform:  runtime.GOOS + "/" + runtime.GOARCH,
	}
}
