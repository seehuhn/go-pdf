// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2025  Jochen Voss <voss@seehuhn.de>
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

package content

// Type identifies the type of content stream.
type Type int

const (
	PageContent    Type = iota // page content stream
	FormContent                // Form XObject (includes annotation appearances)
	PatternContent             // tiling pattern
	Type3Content               // Type 3 font glyph
)

func (ct Type) String() string {
	switch ct {
	case PageContent:
		return "page"
	case FormContent:
		return "form"
	case PatternContent:
		return "pattern"
	case Type3Content:
		return "type3"
	default:
		return "unknown"
	}
}

// Type3Mode tracks d0/d1 mode for Type 3 font glyphs.
type Type3Mode byte

const (
	Type3ModeUnset Type3Mode = iota // d0/d1 not yet called
	Type3ModeD0                     // d0: color operators allowed
	Type3ModeD1                     // d1: color operators forbidden
)

func (m Type3Mode) String() string {
	switch m {
	case Type3ModeUnset:
		return "unset"
	case Type3ModeD0:
		return "d0"
	case Type3ModeD1:
		return "d1"
	default:
		return "unknown"
	}
}
