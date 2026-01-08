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

// Package transition implements PDF page transition dictionaries.
//
// Transition dictionaries control the visual effect used when moving
// from one page to another during a presentation. The transition is
// specified in the destination page's dictionary.
//
// Transition styles include wipes, dissolves, blinds, and various
// other effects. Some styles (Fly, Push, Cover, Uncover, Fade) require
// PDF 1.5 or later.
package transition
