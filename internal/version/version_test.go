package version

import (
	"strings"
	"testing"
)

func TestGetUsesFallbackWhenEmpty(t *testing.T) {
	oldVersion, oldCommit, oldBuildTime := Version, Commit, BuildTime
	t.Cleanup(func() {
		Version, Commit, BuildTime = oldVersion, oldCommit, oldBuildTime
	})

	Version = "   "
	Commit = ""
	BuildTime = " "

	got := Get()
	if got.Version != "dev" {
		t.Fatalf("Version fallback mismatch: %q", got.Version)
	}
	if got.Commit != "none" {
		t.Fatalf("Commit fallback mismatch: %q", got.Commit)
	}
	if got.BuildTime != "unknown" {
		t.Fatalf("BuildTime fallback mismatch: %q", got.BuildTime)
	}
	if got.GoVersion == "" {
		t.Fatal("GoVersion should not be empty")
	}
}

func TestStringIncludesFields(t *testing.T) {
	oldVersion, oldCommit, oldBuildTime := Version, Commit, BuildTime
	t.Cleanup(func() {
		Version, Commit, BuildTime = oldVersion, oldCommit, oldBuildTime
	})

	Version = "v1.2.3"
	Commit = "abc1234"
	BuildTime = "2026-03-17T00:00:00Z"

	out := String()
	if !strings.Contains(out, "Version: v1.2.3") {
		t.Fatalf("missing version in output: %q", out)
	}
	if !strings.Contains(out, "Commit: abc1234") {
		t.Fatalf("missing commit in output: %q", out)
	}
	if !strings.Contains(out, "BuildTime: 2026-03-17T00:00:00Z") {
		t.Fatalf("missing build time in output: %q", out)
	}
	if !strings.Contains(out, "GoVersion: ") {
		t.Fatalf("missing go version in output: %q", out)
	}
}
