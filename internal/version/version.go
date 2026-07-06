// Package version exposes the build version string of the application.
package version

import "runtime/debug"

// String returns the human-readable build version. For release builds it is the
// module version; for local (devel) builds it falls back to the VCS revision,
// suffixed with " dirty" when the working tree had uncommitted changes.
func String() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}

	version := info.Main.Version
	if version == "(devel)" {
		var commit string
		var dirty bool
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" {
				commit = setting.Value
			}
			if setting.Key == "vcs.modified" {
				dirty = setting.Value == "true"
			}
		}
		version += " " + commit
		if dirty {
			version += " dirty"
		}
	}

	return version
}
