// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package extract

import (
	"errors"
	"io"

	"seehuhn.de/go/geom/matrix"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/encoding"
	"seehuhn.de/go/pdf/graphics/content"
)

// extractFontType3 reads a Type 3 font dictionary from a PDF file.
func extractFontType3(c pdf.Cursor, obj pdf.Object) (*dict.Type3, error) {
	fontDict, err := c.DictTyped(obj, "Font")
	if err != nil {
		return nil, err
	} else if fontDict == nil {
		return nil, &pdf.MalformedFileError{
			Err: errors.New("missing font dictionary"),
		}
	}
	subtype, err := c.Name(fontDict["Subtype"])
	if err != nil {
		return nil, err
	}
	if subtype != "" && subtype != "Type3" {
		return nil, pdf.Errorf("expected font subtype Type3, got %q", subtype)
	}

	d := &dict.Type3{}

	d.Name, _ = c.Name(fontDict["Name"])

	fd, err := c.DecodeOptional(fontDict["FontDescriptor"], font.ExtractDescriptor)
	if err != nil {
		return nil, err
	}
	d.Descriptor = fd

	enc, err := c.Decode(fontDict["Encoding"], encoding.ExtractType3)
	if err != nil {
		return nil, err
	}
	d.Encoding = enc

	var defaultWidth float64
	if fd != nil {
		defaultWidth = fd.MissingWidth
	}
	getSimpleWidths(d.Width[:], c, fontDict, defaultWidth)

	d.ToUnicode, _ = c.Decode(fontDict["ToUnicode"], cmap.ExtractToUnicode)

	// Extract CharProcs - parse each content stream
	charProcsDict, err := c.Dict(fontDict["CharProcs"])
	if err != nil {
		return nil, pdf.Wrap(err, "CharProcs")
	}

	// font-level resources; the per-glyph fallback (§7.8.3) is left to
	// the consumer, so that glyph streams without a Resources entry
	// still have none when the font is written back
	if fontDict["Resources"] != nil {
		d.Resources, err = c.Decode(fontDict["Resources"], Resources)
		if pdf.IsReadError(err) {
			return nil, err
		}
	}

	// Names which refer to the same stream share one value, so that each
	// glyph procedure is read, repaired and written only once.
	charProcs := make(map[pdf.Name]*dict.CharProc, len(charProcsDict))
	for name, obj := range charProcsDict {
		cp, err := c.DecodeOptional(obj, decodeCharProc)
		if pdf.IsReadError(err) {
			return nil, err
		} else if err != nil || cp == nil {
			continue // permissive: skip malformed CharProcs
		}
		charProcs[name] = cp
	}
	d.CharProcs = charProcs

	fontBBox, _ := c.Rectangle(fontDict["FontBBox"])
	if fontBBox != nil && !fontBBox.IsZero() {
		d.FontBBox = fontBBox
	}

	d.FontMatrix, _ = c.Matrix(fontDict["FontMatrix"])

	repairType3(d, c.Getter())

	return d, nil
}

// decodeCharProc reads a Type 3 glyph procedure.  It returns nil if obj is
// nil.  The content stream is repaired so that it starts with a d0 or d1
// operator and contains no other (see [content.RepairGlyph]).  This reads the
// stream up to its first usable d0 or d1 operator, or all of it if it has
// none; the remainder is read when the glyph is used.
func decodeCharProc(c pdf.Cursor, obj pdf.Object, _ bool) (*dict.CharProc, error) {
	stm, err := c.Stream(obj)
	if err != nil || stm == nil {
		return nil, err
	}

	var glyphRes *content.Resources
	if stm.Dict["Resources"] != nil {
		glyphRes, err = c.Decode(stm.Dict["Resources"], Resources)
		if pdf.IsReadError(err) {
			return nil, err
		}
	}

	// each iteration re-opens the PDF stream
	stream := content.NewScanner(func() (io.ReadCloser, error) {
		return c.StreamReader(stm)
	})

	return &dict.CharProc{
		Content:   content.RepairGlyph(stream),
		Resources: glyphRes,
	}, nil
}

// repairType3 fixes invalid data in a Type3 font dictionary after extraction.
func repairType3(d *dict.Type3, r pdf.Getter) {
	v := pdf.GetVersion(r)
	if v == pdf.V1_0 {
		if d.Name == "" {
			d.Name = "Font"
		}
	}

	// Drop entries which cannot be written at this PDF version, so that
	// everything we read can be written back.  Each case mirrors a version
	// check in [dict.Type3.Embed].  Readers of the time ignored these
	// entries, so dropping them also matches how such files were displayed.
	if v < pdf.V1_2 {
		d.Resources = nil
	}
	if v < pdf.V2_0 {
		for _, cp := range d.CharProcs {
			cp.Resources = nil
		}
	}
	// Unlike Type 1 / TrueType, Name is preserved in PDF 2.0 for Type 3 fonts
	// because it is the only carrier of the font's human-readable name (Type 3
	// has no BaseFont).  See lrosenthol's clarification at
	// https://github.com/pdf-association/pdf-issues/issues/11#issuecomment-753665847

	if d.FontMatrix.IsZero() {
		d.FontMatrix = matrix.Matrix{0.001, 0, 0, 0.001, 0, 0}
	}
}
