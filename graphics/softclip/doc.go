// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

// Package softclip provides types for PDF soft-mask dictionaries.
//
// Soft masks control the shape or opacity of objects during compositing,
// enabling effects such as gradual transitions between an object and its
// backdrop (soft clipping). The mask values are derived from a transparency
// group XObject, using either the group's alpha channel or its luminosity.
//
// A [Mask] can be set as the current soft mask in the graphics state via the
// SMask entry in an extended graphics state dictionary (ExtGState).
//
// See PDF 2.0 specification sections 11.5 and 11.6.5.1.
package softclip
