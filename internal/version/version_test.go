package version

import (
	"runtime"
	"testing"
)

func TestGetReportsRuntimeDetails(t *testing.T) {
	got := Get()

	if got.GoVersion != runtime.Version() {
		t.Errorf("GoVersion = %q, want %q", got.GoVersion, runtime.Version())
	}
	if want := runtime.GOOS + "/" + runtime.GOARCH; got.Platform != want {
		t.Errorf("Platform = %q, want %q", got.Platform, want)
	}
	if got.Version == "" || got.Commit == "" || got.BuildDate == "" {
		t.Errorf("build metadata must never be empty, got %+v", got)
	}
}

func TestGetUsesInjectedValues(t *testing.T) {
	origVersion, origCommit := Version, Commit
	t.Cleanup(func() { Version, Commit = origVersion, origCommit })

	Version, Commit = "v9.9.9", "deadbeef"

	got := Get()
	if got.Version != "v9.9.9" || got.Commit != "deadbeef" {
		t.Errorf("Get() = %+v, want injected version/commit", got)
	}
}
