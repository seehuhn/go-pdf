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

package image

import (
	"errors"
	"fmt"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/oc"
)

// Alternate is a wrapper dictionary for an alternate image (Table 89).
// Each entry in the Alternates array of a [Dict] or [Mask] is an Alternate.
type Alternate struct {
	// Image is the alternate image XObject (required).
	Image graphics.Image

	// DefaultForPrinting indicates whether this alternate image is the default
	// version to be used for printing. At most one alternate for a given base
	// image may be so designated.
	DefaultForPrinting bool

	// OC (optional; PDF 1.5) is an optional content group or membership
	// dictionary that facilitates the selection of which alternate image to use.
	OC oc.Conditional
}

// Embed writes the alternate image dictionary to the PDF file.
func (a *Alternate) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
	if a.Image == nil {
		return nil, errors.New("missing alternate image")
	}

	ref, err := rm.Embed(a.Image)
	if err != nil {
		return nil, err
	}

	dict := pdf.Dict{
		"Image": ref,
	}

	if a.DefaultForPrinting {
		dict["DefaultForPrinting"] = pdf.Boolean(true)
	}

	if a.OC != nil {
		if err := pdf.CheckVersion(rm.Out(), "alternate image OC entry", pdf.V1_5); err != nil {
			return nil, err
		}
		ocRef, err := rm.Embed(a.OC)
		if err != nil {
			return nil, err
		}
		dict["OC"] = ocRef
	}

	return dict, nil
}

// hasNestedAlternates checks whether a graphics.Image has nested alternates.
func hasNestedAlternates(img graphics.Image) bool {
	switch img := img.(type) {
	case *Dict:
		return len(img.Alternates) > 0
	case *Mask:
		return len(img.Alternates) > 0
	}
	return false
}

// ExtractAlternate extracts an alternate image dictionary from a PDF object.
func ExtractAlternate(c pdf.Cursor, obj pdf.Object, _ bool) (*Alternate, error) {
	dict, err := c.Dict(obj)
	if err != nil {
		return nil, err
	}
	if dict == nil {
		return nil, pdf.Error("missing alternate image dictionary")
	}

	imgObj := dict["Image"]
	if imgObj == nil {
		return nil, pdf.Error("missing Image entry in alternate image dictionary")
	}

	// dispatch based on ImageMask flag
	var img graphics.Image
	stm, err := c.Stream(imgObj)
	if err != nil {
		return nil, fmt.Errorf("invalid Image: %w", err)
	}
	if stm == nil {
		return nil, pdf.Error("missing Image stream in alternate image dictionary")
	}
	if isImageMask, _ := c.Boolean(stm.Dict["ImageMask"]); isImageMask {
		mask, err := pdf.Decode(c, imgObj, ExtractMask)
		if err != nil {
			return nil, fmt.Errorf("invalid Image: %w", err)
		}
		// alternates of alternates not allowed per spec
		mask.Alternates = nil
		img = mask
	} else {
		d, err := pdf.Decode(c, imgObj, ExtractDict)
		if err != nil {
			return nil, fmt.Errorf("invalid Image: %w", err)
		}
		// alternates of alternates not allowed per spec
		d.Alternates = nil
		img = d
	}

	alt := &Alternate{
		Image: img,
	}

	if dfp, err := c.Boolean(dict["DefaultForPrinting"]); err == nil {
		alt.DefaultForPrinting = bool(dfp)
	}

	if ocObj, ok := dict["OC"]; ok {
		if oc, err := pdf.DecodeOptional(c, ocObj, oc.ExtractConditional); err != nil {
			return nil, err
		} else {
			alt.OC = oc
		}
	}

	return alt, nil
}
