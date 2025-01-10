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

package cidfont

import (
	"errors"
	"fmt"
	"math"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/subset"
	"seehuhn.de/go/pdf/font/widths"
	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/cff"
)

// Type0FontData is a font which can be used with a Type 0 CIDFont.
// This must be one of [*cff.Font] or [*sfnt.Font].
type Type0FontData interface{}

// Type0Dict holds the information from the font dictionary and CIDFont dictionary
// of a Type 0 (CFF-based) CIDFont.
type Type0Dict struct {
	// Ref is the reference to the font dictionary in the PDF file.
	Ref pdf.Reference

	// PostScriptName is the PostScript name of the font
	// (without any subset tag).
	PostScriptName string

	SubsetTag string

	// Descriptor is the font descriptor.
	Descriptor *font.Descriptor

	// Encoding specifies how character codes are mapped to CID values.
	Encoding *cmap.InfoNew

	// Width is a map from CID values to glyph widths (in PDF glyph space units).
	Width map[cmap.CID]float64

	// DefaultWidth is the glyph width for CID values not in the Width map
	// (in PDF glyph space units).
	DefaultWidth float64

	// TODO(voss): vertical glyph metrics

	// Text specifies how character codes are mapped to Unicode strings.
	Text *cmap.ToUnicodeInfo

	// GetFont (optional) returns the font data to embed.
	// If this is nil, the font data is not embedded in the PDF file.
	GetFont func() (Type0FontData, error)
}

// Finish embeds the font data in the PDF file.
// This implements the [pdf.Finisher] interface.
func (d *Type0Dict) Finish(rm *pdf.ResourceManager) error {
	w := rm.Out

	var fontData Type0FontData
	if d.GetFont != nil {
		var err error
		fontData, err = d.GetFont()
		if err != nil {
			return err
		}
	}

	// Check that all data are valid and consistent.
	if d.Ref == 0 {
		return errors.New("missing font dictionary reference")
	}
	switch f := fontData.(type) {
	case nil:
		// pass
	case *cff.Font:
		err := pdf.CheckVersion(w, "composite CFF fonts", pdf.V1_3)
		if err != nil {
			return err
		}
	case *sfnt.Font:
		if !f.IsCFF() {
			return errors.New("CFF table missing")
		}
		err := pdf.CheckVersion(w, "composite OpenType/CFF fonts", pdf.V1_6)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported font type %T", fontData)
	}
	if d.SubsetTag != "" && !subset.IsValidTag(d.SubsetTag) {
		return fmt.Errorf("invalid subset tag: %s", d.SubsetTag)
	}

	var cidFontName pdf.Name
	if d.SubsetTag != "" {
		cidFontName = pdf.Name(d.SubsetTag + "+" + d.PostScriptName)
	} else {
		cidFontName = pdf.Name(d.PostScriptName)
	}

	encoding, _, err := pdf.ResourceManagerEmbed(rm, d.Encoding)
	if err != nil {
		return err
	}

	var toUni pdf.Object
	if !d.Text.IsEmpty() {
		toUni, _, err = pdf.ResourceManagerEmbed(rm, d.Text)
		if err != nil {
			return err
		}
	}

	fontDictRef := d.Ref
	cidFontRef := w.Alloc()
	fdRef := w.Alloc()

	fontDict := pdf.Dict{
		"Type":            pdf.Name("Font"),
		"Subtype":         pdf.Name("Type0"),
		"BaseFont":        pdf.Name(cidFontName + "-" + d.Encoding.Name),
		"Encoding":        encoding,
		"DescendantFonts": pdf.Array{cidFontRef},
		"ToUnicode":       toUni,
	}

	cidFontDict := pdf.Dict{
		"Type":           pdf.Name("Font"),
		"Subtype":        pdf.Name("CIDFontType0"),
		"BaseFont":       pdf.Name(cidFontName),
		"CIDSystemInfo":  d.Encoding.ROS,
		"FontDescriptor": fdRef,
		// we set the width information later
	}

	fdDict := d.Descriptor.AsDict()
	fdDict["FontName"] = cidFontDict["BaseFont"]
	var fontFileRef pdf.Reference
	if fontData != nil {
		fontFileRef = w.Alloc()
		fdDict["FontFile3"] = fontFileRef
	}

	compressedObjects := []pdf.Object{fontDict, cidFontDict, fdDict}
	compressedRefs := []pdf.Reference{fontDictRef, cidFontRef, fdRef}

	ww := widths.EncodeComposite2(d.Width, d.DefaultWidth)
	switch {
	case len(ww) > 10:
		wwRef := w.Alloc()
		cidFontDict["W"] = wwRef
		compressedObjects = append(compressedObjects, ww)
		compressedRefs = append(compressedRefs, wwRef)
	case len(ww) != 0:
		cidFontDict["W"] = ww
	}
	if math.Abs(d.DefaultWidth-1000) > 0.01 {
		cidFontDict["DW"] = pdf.Number(d.DefaultWidth)
	}

	err = w.WriteCompressed(compressedRefs, compressedObjects...)
	if err != nil {
		return pdf.Wrap(err, "composite OpenType/CFF font dicts")
	}

	// See section 9.9 of PDF 32000-1:2008 for details.
	switch f := fontData.(type) {
	case *cff.Font:
		fontFileDict := pdf.Dict{
			"Subtype": pdf.Name("CIDFontType0C"),
		}
		fontFileStream, err := w.OpenStream(fontFileRef, fontFileDict, pdf.FilterCompress{})
		if err != nil {
			return err
		}
		err = f.Write(fontFileStream)
		if err != nil {
			return fmt.Errorf("CFF font program %q: %w", cidFontName, err)
		}
		err = fontFileStream.Close()
		if err != nil {
			return err
		}

	case *sfnt.Font:
		fontFileDict := pdf.Dict{
			"Subtype": pdf.Name("OpenType"),
		}
		fontFileStream, err := w.OpenStream(fontFileRef, fontFileDict, pdf.FilterCompress{})
		if err != nil {
			return err
		}
		err = f.WriteOpenTypeCFFPDF(fontFileStream)
		if err != nil {
			return fmt.Errorf("OpenType/CFF font program %q: %w", cidFontName, err)
		}
		err = fontFileStream.Close()
		if err != nil {
			return err
		}
	}

	return nil
}
