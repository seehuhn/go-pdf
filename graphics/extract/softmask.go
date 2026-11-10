// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package extract

import (
	"errors"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/function"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/softclip"
)

// SoftMaskDict reads a soft-mask dictionary from a PDF file.
// Returns nil without error if obj is nil or /None.
// Note: For soft-mask images, use [SoftMaskImage] instead.
func SoftMaskDict(c pdf.Cursor, obj pdf.Object, _ bool) (graphics.SoftClip, error) {
	if obj == nil {
		return nil, nil
	}

	// Check for /None name (explicit absence of soft mask)
	resolved, err := c.Resolve(obj)
	if err != nil {
		return nil, err
	}
	if resolved == pdf.Name("None") {
		return nil, nil
	}

	dict, err := c.Dict(obj)
	if err != nil {
		return nil, err
	}
	if dict == nil {
		return nil, nil
	}

	m := &softclip.Mask{}

	// S - subtype (required)
	sName, err := c.Name(dict["S"])
	if err != nil {
		return nil, err
	}
	switch sName {
	case "Alpha":
		m.S = softclip.Alpha
	case "Luminosity":
		m.S = softclip.Luminosity
	default:
		return nil, &pdf.MalformedFileError{
			Err: errors.New("invalid soft mask subtype: " + string(sName)),
		}
	}

	// G - transparency group XObject (required)
	gObj := dict["G"]
	if gObj == nil {
		return nil, &pdf.MalformedFileError{
			Err: errors.New("soft mask: missing G entry"),
		}
	}
	g, err := c.Decode(gObj, Form)
	if err != nil {
		return nil, err
	}
	if g.Group == nil {
		return nil, &pdf.MalformedFileError{
			Err: errors.New("soft mask: G must be a transparency group XObject"),
		}
	}
	m.G = g

	// BC - backdrop color (optional, Luminosity only)
	if m.S == softclip.Luminosity {
		bc, err := c.FloatArray(dict["BC"])
		if pdf.IsReadError(err) {
			return nil, err
		}
		if cs := g.Group.CS; cs != nil && len(bc) == cs.Channels() {
			m.BC = bc
		}
	}

	// TR - transfer function (optional)
	if trObj := dict["TR"]; trObj != nil {
		// Check for /Identity name
		trResolved, err := c.Resolve(trObj)
		if err != nil {
			return nil, err
		}
		if trResolved != pdf.Name("Identity") {
			tr, err := c.DecodeOptional(trObj, function.Extract)
			if err != nil {
				return nil, err
			}
			m.TR = tr
		}
		// If /Identity, m.TR stays nil (which represents identity)
	}

	return m, nil
}
