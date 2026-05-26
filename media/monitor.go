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

package media

// MonitorSpecifier identifies a physical monitor attached to the system.
type MonitorSpecifier int

// Valid values for the [MonitorSpecifier] type.
const (
	MonitorLargestDocument  MonitorSpecifier = 0 // largest section of the document window
	MonitorSmallestDocument MonitorSpecifier = 1 // smallest section of the document window
	MonitorPrimary          MonitorSpecifier = 2 // primary monitor
	MonitorGreatestDepth    MonitorSpecifier = 3 // greatest colour depth
	MonitorGreatestArea     MonitorSpecifier = 4 // greatest area in pixels squared
	MonitorGreatestHeight   MonitorSpecifier = 5 // greatest height in pixels
	MonitorGreatestWidth    MonitorSpecifier = 6 // greatest width in pixels
)

// isValid reports whether m is a recognised monitor specifier value.
func (m MonitorSpecifier) isValid() bool {
	return m >= MonitorLargestDocument && m <= MonitorGreatestWidth
}
