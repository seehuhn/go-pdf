// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

// Package glyphdata encodes and decodes font data streams in PDF files.
//
// This package handles different font formats including Type1, TrueType, CFF,
// and OpenType fonts. Each format has its own subpackage that implements
// format-specific embedding and extraction logic.
//
// The main Type enumeration defines the supported font types, and each
// subpackage provides Embed and Extract functions for handling font data
// streams in PDF files.
//
// # Dependency boundary
//
// The format subpackages (type1glyphs, cffglyphs, sfntglyphs) import the
// respective binary-font parsers.  PDF-only consumers — font/encoding,
// font/dict, graphics/extract — keep clear of these subpackages so that
// programs which only inspect PDF structure need not compile in the full
// font-parsing machinery.
package glyphdata
