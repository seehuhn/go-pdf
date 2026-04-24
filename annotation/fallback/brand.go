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

package fallback

import "seehuhn.de/go/pdf/graphics/color"

// Quire design-system palette, used as the default colours for fallback
// appearance streams. Names and values mirror the CSS custom properties
// in the Quire design system (colors_and_type.css).

var (
	// ink neutrals
	quireInk  = color.DeviceRGB{0.102, 0.094, 0.078} // --ink   #1a1814
	quireInk2 = color.DeviceRGB{0.227, 0.216, 0.184} // --ink-2 #3a372f
	quireInk3 = color.DeviceRGB{0.427, 0.408, 0.345} // --ink-3 #6d6858

	// slate (cool neutrals — app chrome on warm paper)
	quireSlate1 = color.DeviceRGB{0.961, 0.961, 0.969} // --slate-1 #f5f5f7
	quireSlate3 = color.DeviceRGB{0.851, 0.851, 0.867} // --slate-3 #d9d9dd

	// amber accents (editorial only)
	quireAmber100 = color.DeviceRGB{0.965, 0.898, 0.749} // --amber-100 #f6e5bf
	quireAmber400 = color.DeviceRGB{0.761, 0.459, 0.094} // --amber-400 #c27518

	// lapis (academic blue — hyperlinks in prose)
	quireLapis500 = color.DeviceRGB{0.141, 0.353, 0.620} // --lapis-500 #245a9e

	// signals
	quireSignalError = color.DeviceRGB{0.604, 0.173, 0.173} // --signal-error #9a2c2c
)
