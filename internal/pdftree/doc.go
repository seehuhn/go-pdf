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

// Package pdftree implements the shared B-tree machinery behind PDF name trees
// and number trees (PDF 32000-2 sections 7.9.6 and 7.9.7).
//
// The two kinds of tree differ only in their key type and in how keys are
// stored in the file.  That variation is captured by the unexported codec
// interface, with [NameCodec] and [NumCodec] as its two instantiations.  The
// seehuhn.de/go/pdf/nametree and seehuhn.de/go/pdf/numtree packages are thin
// facades over this core.
package pdftree
