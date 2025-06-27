package cmd

import (
	"fmt"
	"runtime"
	"runtime/debug"
)

var (
	// Build-time variables set by ldflags
	Version   = "dev"
	GitCommit = "unknown"
	GitTag    = "unknown"
)

// GetVersionInfo returns version information
func GetVersionInfo() string {
	var version string
	var commit string

	// Use goreleaser build-time variables if available
	if GitTag != "unknown" && GitTag != "" {
		version = GitTag
	} else if Version != "dev" && Version != "" {
		version = Version
	} else {
		version = "dev"

		// Try to get version info from build info when using go install
		if buildInfo, ok := debug.ReadBuildInfo(); ok {
			if buildInfo.Main.Version != "" && buildInfo.Main.Version != "(devel)" {
				version = buildInfo.Main.Version
			} else {
				// Check for VCS info
				for _, setting := range buildInfo.Settings {
					switch setting.Key {
					case "vcs.revision":
						if len(setting.Value) > 7 {
							version = fmt.Sprintf("dev-%s", setting.Value[:7])
						} else {
							version = fmt.Sprintf("dev-%s", setting.Value)
						}
					case "vcs.time":
						if version == "dev" || version == Version {
							version = fmt.Sprintf("dev-%s", setting.Value[:10])
						}
					}
				}
			}
		}
	}

	// Add commit info if available
	if GitCommit != "unknown" && GitCommit != "" && len(GitCommit) > 7 {
		commit = fmt.Sprintf(" (commit: %s)", GitCommit[:7])
	}

	return fmt.Sprintf("kubectl-setimg version %s%s\nGo version: %s", version, commit, runtime.Version())
}
