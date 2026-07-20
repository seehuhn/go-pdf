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

// Package printprep rewrites a PDF document into a print-safe simplification.
//
// The output is a fresh, unencrypted PDF that contains only the visible,
// printable content of the source, using font constructions that render
// reliably across a broad range of PDF renderers and print pipelines.  It is
// intended to be handed to a host print system for rasterization.
//
// The transformation:
//
//   - normalizes fonts so that renderers do not have to interpret unusual
//     font constructions (see the font policy in [Write]);
//   - removes the content of optional-content groups that are switched off,
//     and drops the optional-content machinery entirely;
//   - flattens printable annotations into page content and drops
//     interactive, media and hidden annotations;
//   - strips tagged-PDF structure, metadata, navigation, and other data that
//     does not affect the printed marks;
//   - writes a fresh, unencrypted document.
//
// The package is platform-independent; it knows nothing about any particular
// operating system or renderer.
package printprep
