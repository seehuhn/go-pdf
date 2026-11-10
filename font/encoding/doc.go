// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

// Package encoding interprets the /Encoding entries of simple (single-byte)
// PDF font dictionaries.
//
// This package does not parse embedded font binaries.  Codes mapped to
// [UseBuiltin] require a font-format-specific resolver from a subpackage
// of font/glyphdata.
package encoding
