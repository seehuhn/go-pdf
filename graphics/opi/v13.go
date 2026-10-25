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
	"fmt"
	"math"
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
	Comments pdf.TextString

	// Size is the image dimensions in pixels, as [pixelsWide pixelsHigh].
	Size [2]int

	// CropRect is the portion of the image to use, as [left top right bottom],
	// with 0 ≤ left < right ≤ width.  Top and bottom must differ and lie in
	// the range 0 to height, in either order.
	CropRect [4]int

	// CropFixed (optional) is CropRect expressed with real numbers.  It must
	// satisfy the same conditions as CropRect.
	CropFixed *[4]float64

	// Position is the location of the cropped image on the page, as eight
	// numbers [llx lly ulx uly urx ury lrx lry].  The four corners must be
	// finite and form a parallelogram.
	Position [8]float64

	// Resolution (optional) is the image resolution in samples per inch, as
	// [horizRes vertRes].
	Resolution *[2]float64

	// ColorType (optional) is the type of colour given by Color; one of
	// Process, Spot, or Separation.
	ColorType pdf.Name

	// Color (optional) is the colour the image is rendered in.  The CMYK
	// values must be in the range 0.0 to 1.0.
	Color *Color13

	// Tint (optional) is the concentration of Color, in the range 0.0 to 1.0.
	Tint *float64

	// Overprint specifies whether the image overprints underlying marks.
	Overprint bool

	// ImageType (optional) is the image format as [samples bits].
	ImageType *[2]int

	// GrayMap (optional) records brightness or contrast changes, as an array
	// of 2^n integers in the range 0 to 65535, where n is the number of bits
	// per sample.  If ImageType is set, n must match its bits per sample.
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
	Text pdf.TextString
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
	if !validCrop13(&v.Size, &v.CropRect) {
		return nil, pdf.Error("OPI 1.3 CropRect must lie inside the image")
	}
	if v.CropFixed != nil && !validCrop13(sizeFloat13(v.Size), v.CropFixed) {
		return nil, pdf.Error("OPI 1.3 CropFixed must lie inside the image")
	}
	if !validPosition(&v.Position) {
		return nil, pdf.Error("OPI 1.3 Position must be finite and form a parallelogram")
	}
	if !validColorType13(v.ColorType) {
		return nil, fmt.Errorf("invalid OPI 1.3 ColorType %q", v.ColorType)
	}
	if v.Color != nil {
		for _, x := range v.Color.CMYK {
			if !inUnitRange(x) {
				return nil, pdf.Error("OPI 1.3 Color values must be in the range 0 to 1")
			}
		}
	}
	if v.Tint != nil && !inUnitRange(*v.Tint) {
		return nil, pdf.Error("OPI 1.3 Tint must be in the range 0 to 1")
	}
	if len(v.GrayMap) > 0 && !validGrayMap(v.GrayMap, v.ImageType) {
		return nil, pdf.Error("invalid OPI 1.3 GrayMap")
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
	if !v.Comments.IsZero() {
		inner["Comments"] = v.Comments
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
			arr = append(arr, pdf.Integer(t.Num), t.Text)
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

	fs, err := c.Decode(dict["F"], file.ExtractSpecification)
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
	if cm, err := pdf.Optional(c.TextString(dict["Comments"])); err != nil {
		return nil, err
	} else if !cm.IsZero() {
		v.Comments = cm
	}

	// Size, CropRect and Position are required.  CropFixed has the same
	// meaning as CropRect, so a missing or invalid CropRect can be recovered
	// from it.  The others cannot be repaired.
	if s, err := readInts(c, dict["Size"]); err != nil {
		return nil, err
	} else if len(s) == 2 {
		v.Size = [2]int{s[0], s[1]}
	} else {
		return nil, pdf.Error("OPI 1.3 dictionary has no valid Size entry")
	}
	if cf, err := readNumbers(c, dict["CropFixed"]); err != nil {
		return nil, err
	} else if len(cf) == 4 {
		crop := [4]float64(cf)
		if validCrop13(sizeFloat13(v.Size), &crop) {
			v.CropFixed = &crop
		}
	}
	if cr, err := readInts(c, dict["CropRect"]); err != nil {
		return nil, err
	} else if len(cr) == 4 && validCrop13(&v.Size, (*[4]int)(cr)) {
		v.CropRect = [4]int(cr)
	} else if v.CropFixed != nil {
		v.CropRect = roundOut13(v.CropFixed)
	} else {
		return nil, pdf.Error("OPI 1.3 dictionary has no valid CropRect entry")
	}
	if p, err := readNumbers(c, dict["Position"]); err != nil {
		return nil, err
	} else if len(p) == 8 {
		v.Position = [8]float64(p)
		if !validPosition(&v.Position) {
			// move the lower-right corner to complete the parallelogram
			v.Position[6] = v.Position[0] + v.Position[4] - v.Position[2]
			v.Position[7] = v.Position[1] + v.Position[5] - v.Position[3]
			if !validPosition(&v.Position) {
				return nil, pdf.Error("OPI 1.3 dictionary has no valid Position entry")
			}
		}
	} else {
		return nil, pdf.Error("OPI 1.3 dictionary has no valid Position entry")
	}

	if r, err := readNumbers(c, dict["Resolution"]); err != nil {
		return nil, err
	} else if len(r) == 2 {
		v.Resolution = &[2]float64{r[0], r[1]}
	}
	if ct, err := pdf.Optional(c.Name(dict["ColorType"])); err != nil {
		return nil, err
	} else if validColorType13(ct) {
		v.ColorType = ct
	}
	if col, err := readColor13(c, dict["Color"]); err != nil {
		return nil, err
	} else {
		v.Color = col
	}
	if t, err := c.Number(dict["Tint"]); err == nil {
		if inUnitRange(t) {
			v.Tint = &t
		}
	} else if pdf.IsReadError(err) {
		return nil, err
	}
	o, _, err := c.OptionalBoolean(dict["Overprint"])
	if err != nil {
		return nil, err
	}
	v.Overprint = o
	if it, err := readInts(c, dict["ImageType"]); err != nil {
		return nil, err
	} else if len(it) == 2 {
		v.ImageType = &[2]int{it[0], it[1]}
	}
	if gm, err := readInts(c, dict["GrayMap"]); err != nil {
		return nil, err
	} else if validGrayMap(gm, v.ImageType) {
		v.GrayMap = gm
	}
	if tr, ok, err := c.OptionalBoolean(dict["Transparency"]); err != nil {
		return nil, err
	} else if ok {
		v.Transparency = &tr
	}
	if tags, err := pdf.Optional(c.Array(dict["Tags"])); err != nil {
		return nil, err
	} else {
		// each pair stands on its own, so a malformed pair is skipped
		for i := 0; i+1 < len(tags); i += 2 {
			num, err := c.Integer(tags[i])
			if pdf.IsMalformed(err) {
				continue
			} else if err != nil {
				return nil, err
			}
			text, err := c.TextString(tags[i+1])
			if pdf.IsMalformed(err) {
				continue
			} else if err != nil {
				return nil, err
			}
			v.Tags = append(v.Tags, Tag13{Num: int(num), Text: text})
		}
	}

	return v, nil
}

// readColor13 reads the Color entry of an OPI 1.3 dictionary.  The result is
// nil if the entry is absent or malformed.
func readColor13(c pdf.Cursor, obj pdf.Object) (*Color13, error) {
	arr, err := pdf.Optional(c.Array(obj))
	if err != nil || len(arr) != 5 {
		return nil, err
	}
	cmyk, err := readNumbers(c, arr[:4])
	if err != nil || cmyk == nil {
		return nil, err
	}
	for _, x := range cmyk {
		if !inUnitRange(x) {
			return nil, nil
		}
	}
	name, err := c.String(arr[4])
	if pdf.IsMalformed(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	return &Color13{CMYK: [4]float64(cmyk), Name: name}, nil
}

// validColorType13 reports whether ct is a valid OPI 1.3 ColorType, with the
// empty name standing for an absent entry.
func validColorType13(ct pdf.Name) bool {
	switch ct {
	case "", "Process", "Spot", "Separation":
		return true
	default:
		return false
	}
}

// sizeFloat13 converts an OPI 1.3 Size entry to floating point, for checking
// CropFixed against it.
func sizeFloat13(size [2]int) *[2]float64 {
	return &[2]float64{float64(size[0]), float64(size[1])}
}

// roundOut13 rounds an OPI 1.3 crop rectangle outwards to integers, keeping
// the order of the vertical coordinates.  If crop lies inside an image, so
// does the result, and the result is never empty.
func roundOut13(crop *[4]float64) [4]int {
	res := [4]int{int(math.Floor(crop[0])), 0, int(math.Ceil(crop[2])), 0}
	if crop[1] <= crop[3] {
		res[1], res[3] = int(math.Floor(crop[1])), int(math.Ceil(crop[3]))
	} else {
		res[1], res[3] = int(math.Ceil(crop[1])), int(math.Floor(crop[3]))
	}
	return res
}
