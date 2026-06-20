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

package opi

import (
	"reflect"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/file"
)

// V13 is an OPI version 1.3 dictionary (Table 406). OPI 1.3 refers to the
// version of the Open Prepress Interface specification, not a PDF version.
type V13 struct {
	// F is the external file containing the image this proxy stands for.
	F *file.Specification

	// ID (optional) is an identifying string for the image.
	ID pdf.String

	// Comments (optional) is a human-readable comment for the OPI server
	// operator.
	Comments string

	// Size is the image dimensions in pixels, as [pixelsWide pixelsHigh].
	Size [2]int

	// CropRect is the portion of the image to use, as [left top right bottom].
	CropRect [4]int

	// CropFixed (optional) is CropRect expressed with real numbers.
	CropFixed *[4]float64

	// Position is the location of the cropped image on the page, as eight
	// numbers [llx lly ulx uly urx ury lrx lry].
	Position [8]float64

	// Resolution (optional) is the image resolution in samples per inch, as
	// [horizRes vertRes].
	Resolution *[2]float64

	// ColorType (optional) is the type of colour given by Color; one of
	// Process, Spot, or Separation.
	ColorType pdf.Name

	// Color (optional) is the colour the image is rendered in.
	Color *Color13

	// Tint (optional) is the concentration of Color, in the range 0.0 to 1.0.
	Tint *float64

	// Overprint specifies whether the image overprints underlying marks.
	Overprint bool

	// ImageType (optional) is the image format as [samples bits].
	ImageType *[2]int

	// GrayMap (optional) records brightness or contrast changes, as an array
	// of integers in the range 0 to 65535.
	GrayMap []int

	// Transparency (optional) specifies whether white image pixels are
	// treated as transparent.
	Transparency *bool

	// Tags (optional) is a list of TIFF tag number and ASCII value pairs.
	Tags []Tag13

	// SingleUse determines if Embed stores the inner OPI dictionary inline
	// (true) or as an indirect object (false).
	SingleUse bool
}

// Color13 is the colour entry of an OPI 1.3 dictionary: CMYK values together
// with a colourant name.
type Color13 struct {
	CMYK [4]float64
	Name pdf.String
}

// Tag13 is a TIFF tag number paired with its ASCII value.
type Tag13 struct {
	Num  int
	Text string
}

var _ Dict = (*V13)(nil)

func (*V13) isOPI() {}

// Equal reports whether v and other are equal OPI dictionaries.
func (v *V13) Equal(other Dict) bool {
	o, ok := other.(*V13)
	if !ok {
		return false
	}
	return reflect.DeepEqual(v, o)
}

// Embed adds the OPI 1.3 dictionary to a PDF file, wrapped in an OPI version
// dictionary.
//
// This implements the [pdf.Embedder] interface.
func (v *V13) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out(), "OPI dictionary", pdf.V1_2); err != nil {
		return nil, err
	}
	if v.F == nil {
		return nil, pdf.Error("OPI dictionary requires F entry")
	}

	inner := pdf.Dict{"Version": pdf.Number(1.3)}
	if rm.Out().GetOptions().HasAny(pdf.OptDictTypes) {
		inner["Type"] = pdf.Name("OPI")
	}

	fObj, err := rm.Embed(v.F)
	if err != nil {
		return nil, err
	}
	inner["F"] = fObj

	if len(v.ID) > 0 {
		inner["ID"] = v.ID
	}
	if v.Comments != "" {
		inner["Comments"] = pdf.TextString(v.Comments)
	}
	inner["Size"] = intsToArray(v.Size[:])
	inner["CropRect"] = intsToArray(v.CropRect[:])
	if v.CropFixed != nil {
		inner["CropFixed"] = numbersToArray(v.CropFixed[:])
	}
	inner["Position"] = numbersToArray(v.Position[:])
	if v.Resolution != nil {
		inner["Resolution"] = numbersToArray(v.Resolution[:])
	}
	if v.ColorType != "" {
		inner["ColorType"] = v.ColorType
	}
	if v.Color != nil {
		inner["Color"] = pdf.Array{
			pdf.Number(v.Color.CMYK[0]), pdf.Number(v.Color.CMYK[1]),
			pdf.Number(v.Color.CMYK[2]), pdf.Number(v.Color.CMYK[3]),
			v.Color.Name,
		}
	}
	if v.Tint != nil {
		inner["Tint"] = pdf.Number(*v.Tint)
	}
	if v.Overprint {
		inner["Overprint"] = pdf.Boolean(true)
	}
	if v.ImageType != nil {
		inner["ImageType"] = intsToArray(v.ImageType[:])
	}
	if len(v.GrayMap) > 0 {
		inner["GrayMap"] = intsToArray(v.GrayMap)
	}
	if v.Transparency != nil {
		inner["Transparency"] = pdf.Boolean(*v.Transparency)
	}
	if len(v.Tags) > 0 {
		arr := make(pdf.Array, 0, 2*len(v.Tags))
		for _, t := range v.Tags {
			arr = append(arr, pdf.Integer(t.Num), pdf.TextString(t.Text))
		}
		inner["Tags"] = arr
	}

	return embedVersion(rm, "1.3", inner, v.SingleUse)
}

func extractV13(c pdf.Cursor, obj pdf.Object, isDirect bool) (*V13, error) {
	dict, err := c.DictTyped(obj, "OPI")
	if err != nil {
		return nil, err
	} else if dict == nil {
		return nil, pdf.Error("missing OPI dictionary")
	}

	v := &V13{SingleUse: isDirect}

	fs, err := pdf.Decode(c, dict["F"], file.ExtractSpecification)
	if err != nil {
		return nil, err
	} else if fs == nil {
		return nil, pdf.Error("OPI dictionary requires F entry")
	}
	v.F = fs

	if id, err := pdf.Optional(c.String(dict["ID"])); err != nil {
		return nil, err
	} else if len(id) > 0 {
		v.ID = id
	}
	if c, err := pdf.Optional(c.String(dict["Comments"])); err != nil {
		return nil, err
	} else if len(c) > 0 {
		v.Comments = string(c.AsTextString())
	}
	if s, err := readInts(c, dict["Size"]); err != nil {
		return nil, err
	} else if len(s) == 2 {
		v.Size = [2]int{s[0], s[1]}
	}
	if c, err := readInts(c, dict["CropRect"]); err != nil {
		return nil, err
	} else if len(c) == 4 {
		v.CropRect = [4]int{c[0], c[1], c[2], c[3]}
	}
	if c, err := readNumbers(c, dict["CropFixed"]); err != nil {
		return nil, err
	} else if len(c) == 4 {
		v.CropFixed = &[4]float64{c[0], c[1], c[2], c[3]}
	}
	if p, err := readNumbers(c, dict["Position"]); err != nil {
		return nil, err
	} else if len(p) == 8 {
		v.Position = [8]float64{p[0], p[1], p[2], p[3], p[4], p[5], p[6], p[7]}
	}
	if r, err := readNumbers(c, dict["Resolution"]); err != nil {
		return nil, err
	} else if len(r) == 2 {
		v.Resolution = &[2]float64{r[0], r[1]}
	}
	if ct, err := pdf.Optional(c.Name(dict["ColorType"])); err != nil {
		return nil, err
	} else {
		v.ColorType = ct
	}
	if col, err := pdf.Optional(c.Array(dict["Color"])); err != nil {
		return nil, err
	} else if len(col) == 5 {
		var col13 Color13
		for i := range 4 {
			n, err := pdf.Optional(c.Number(col[i]))
			if err != nil {
				return nil, err
			}
			col13.CMYK[i] = n
		}
		name, err := pdf.Optional(c.String(col[4]))
		if err != nil {
			return nil, err
		}
		col13.Name = name
		v.Color = &col13
	}
	if _, ok := dict["Tint"]; ok {
		if t, err := pdf.Optional(c.Number(dict["Tint"])); err != nil {
			return nil, err
		} else {
			tint := t
			v.Tint = &tint
		}
	}
	if o, err := pdf.Optional(c.Boolean(dict["Overprint"])); err != nil {
		return nil, err
	} else {
		v.Overprint = bool(o)
	}
	if it, err := readInts(c, dict["ImageType"]); err != nil {
		return nil, err
	} else if len(it) == 2 {
		v.ImageType = &[2]int{it[0], it[1]}
	}
	if gm, err := readInts(c, dict["GrayMap"]); err != nil {
		return nil, err
	} else if len(gm) > 0 {
		v.GrayMap = gm
	}
	if _, ok := dict["Transparency"]; ok {
		if tr, err := pdf.Optional(c.Boolean(dict["Transparency"])); err != nil {
			return nil, err
		} else {
			b := bool(tr)
			v.Transparency = &b
		}
	}
	if tags, err := pdf.Optional(c.Array(dict["Tags"])); err != nil {
		return nil, err
	} else if len(tags) >= 2 {
		for i := 0; i+1 < len(tags); i += 2 {
			num, err := pdf.Optional(c.Integer(tags[i]))
			if err != nil {
				return nil, err
			}
			text, err := pdf.Optional(c.String(tags[i+1]))
			if err != nil {
				return nil, err
			}
			v.Tags = append(v.Tags, Tag13{Num: int(num), Text: string(text.AsTextString())})
		}
	}

	return v, nil
}
