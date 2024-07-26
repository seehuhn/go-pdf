// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2023  Jochen Voss <voss@seehuhn.de>
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

package font

import (
	"golang.org/x/text/language"
	"seehuhn.de/go/pdf/font/cmap"
)

// Options allows to customize fonts for embedding into PDF files.
// Not all fields apply to all font types.
type Options struct {
	Language language.Tag

	GsubFeatures map[string]bool
	GposFeatures map[string]bool

	// Composite specifies whether to embed the font as a composite font.
	Composite bool

	MakeGIDToCID func() cmap.GIDToCID                // only used for composite fonts
	MakeEncoder  func(cmap.GIDToCID) cmap.CIDEncoder // only used for composite fonts
}
