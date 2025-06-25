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
	if GitTag != "unknown" && GitTag != "" {
		version = GitTag
	} else if GitCommit != "unknown" && GitCommit != "" {
		version = GitCommit
	} else {
		version = Version

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

	return fmt.Sprintf("kubectl-setimg version %s\nGo version: %s", version, runtime.Version())
}
