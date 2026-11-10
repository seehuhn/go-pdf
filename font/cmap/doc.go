// Copyright 2023 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

// Package cmap implements CMap files for embedding in PDF files.
//
// When composite fonts are used in PDF files, glyphs are selected using a
// two-step process: first, character codes are mapped to character identifiers
// (CIDs), and then CIDs are mapped to glyph identifiers (GIDs). A CMap file
// describes the first step of this process, i.e. the mapping from character
// codes to CIDs.
package cmap
