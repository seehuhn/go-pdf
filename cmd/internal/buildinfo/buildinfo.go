// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2026  Jochen Voss <voss@seehuhn.de>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

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
