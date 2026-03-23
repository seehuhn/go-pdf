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

// Package oc implements optional content (PDF 1.5, section 8.11),
// the mechanism behind layers in PDF documents. Optional content
// allows parts of a document to be selectively viewed or hidden.
//
// The main types are:
//
//   - [Group] is a named toggle that controls visibility of associated content.
//   - [Membership] computes visibility from a boolean combination of groups.
//   - [Configuration] defines initial group states and UI presentation.
//   - [Properties] is the top-level container stored in the document catalog.
//
// A [Group] or [Membership] (both implement [Conditional]) is associated
// with content via marked content sequences (BDC/EMC) in content streams.
//
// Group visibility is tracked as a [GroupStates] value. Use
// [Configuration.DefaultState] to compute initial states from a configuration.
// Groups whose intent does not match the configuration are excluded from the
// state, so they have no effect on visibility.
package oc
