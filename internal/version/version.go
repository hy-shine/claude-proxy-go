package version

import (
	"fmt"
	"runtime"
	"strings"
)

var (
	// Version is the semantic version of this binary.
	// It can be overridden at build time via -ldflags -X.
	Version = "dev"
	// Commit is the git commit hash used to build this binary.
	// It can be overridden at build time via -ldflags -X.
	Commit = "none"
	// BuildTime is the UTC build timestamp for this binary.
	// It can be overridden at build time via -ldflags -X.
	BuildTime = "unknown"
)

type Info struct {
	Version   string
	Commit    string
	BuildTime string
	GoVersion string
}

func Get() Info {
	return Info{
		Version:   fallback(Version, "dev"),
		Commit:    fallback(Commit, "none"),
		BuildTime: fallback(BuildTime, "unknown"),
		GoVersion: runtime.Version(),
	}
}

func String() string {
	info := Get()
	return fmt.Sprintf(
		"Version: %s\nCommit: %s\nBuildTime: %s\nGoVersion: %s",
		info.Version,
		info.Commit,
		info.BuildTime,
		info.GoVersion,
	)
}

func fallback(v, d string) string {
	if strings.TrimSpace(v) == "" {
		return d
	}
	return v
}
