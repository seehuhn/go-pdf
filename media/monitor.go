// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

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
