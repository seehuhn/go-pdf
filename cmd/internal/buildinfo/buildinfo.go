// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package buildinfo

import (
	"runtime/debug"
)

// Short returns a short version string for a CLI tool, e.g.
// "pdf-inspect (seehuhn.de/go/pdf v0.1.0)".
func Short(toolName string) string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return toolName
	}

	version := info.Main.Version
	if version != "" && version != "(devel)" {
		return toolName + " (" + info.Main.Path + " " + version + ")"
	}

	// fall back to VCS revision
	var rev string
	var dirty bool
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			dirty = s.Value == "true"
		}
	}
	if rev == "" {
		return toolName
	}
	if len(rev) > 8 {
		rev = rev[:8]
	}
	if dirty {
		rev += "+dirty"
	}
	return toolName + " (" + info.Main.Path + " " + rev + ")"
}
