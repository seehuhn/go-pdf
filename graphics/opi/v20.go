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

// V20 is an OPI version 2.0 dictionary (Table 407). OPI 2.0 refers to the
// version of the Open Prepress Interface specification, not a PDF version.
type V20 struct {
	// F is the external file containing the low-resolution proxy image.
	F *file.Specification

	// MainImage (optional) identifies the full-resolution image, typically by
	// pathname.
	MainImage pdf.String

	// Tags (optional) is a list of TIFF tag number and ASCII value pairs.
	Tags []Tag20

	// Size (optional) is the image dimensions in pixels, as [width height].
	// Size and CropRect are present or absent together.
	Size *[2]float64

	// CropRect (optional) is the portion of the image to use, as
	// [left top right bottom].
	CropRect *[4]float64

	// Overprint specifies whether the image overprints underlying marks.
	Overprint bool

	// Inks (optional) specifies the colourants applied to the image.
	Inks *Inks20

	// IncludedImageDimensions (optional) is the included image size in pixels,
	// as [pixelsWide pixelsHigh].
	IncludedImageDimensions *[2]int

	// IncludedImageQuality (optional) is the quality of the included image;
	// valid values are 1, 2, and 3. Zero means the entry is absent.
	IncludedImageQuality float64

	// SingleUse determines if Embed stores the inner OPI dictionary inline
	// (true) or as an indirect object (false).
	SingleUse bool
}

// Tag20 is a TIFF tag number paired with one or more ASCII values.
type Tag20 struct {
	Num  int
	Text []string
}

// Inks20 specifies the colourants applied to an OPI 2.0 proxy image. Either
// Name (full_color or registration) or Monochrome is set.
type Inks20 struct {
	// Name is full_color or registration.
	Name pdf.Name

	// Monochrome lists colourant name and tint pairs.
	Monochrome []Ink20Comp
}

// Ink20Comp is a colourant name and the tint applied to it.
type Ink20Comp struct {
	Name pdf.String
	Tint float64
}

var _ Dict = (*V20)(nil)

func (*V20) isOPI() {}

// Equal reports whether v and other are equal OPI dictionaries.
func (v *V20) Equal(other Dict) bool {
	o, ok := other.(*V20)
	if !ok {
		return false
	}
	return reflect.DeepEqual(v, o)
}

// Embed adds the OPI 2.0 dictionary to a PDF file, wrapped in an OPI version
// dictionary.
//
// This implements the [pdf.Embedder] interface.
func (v *V20) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out(), "OPI dictionary", pdf.V1_2); err != nil {
		return nil, err
	}
	if v.F == nil {
		return nil, pdf.Error("OPI dictionary requires F entry")
	}
	if (v.Size == nil) != (v.CropRect == nil) {
		return nil, pdf.Error("OPI 2.0 Size and CropRect must both be set or unset")
	}
	if q := v.IncludedImageQuality; q != 0 && q != 1 && q != 2 && q != 3 {
		return nil, pdf.Error("OPI 2.0 IncludedImageQuality must be 1, 2, or 3")
	}

	inner := pdf.Dict{"Version": pdf.Number(2.0)}
	if rm.Out().GetOptions().HasAny(pdf.OptDictTypes) {
		inner["Type"] = pdf.Name("OPI")
	}

	fObj, err := rm.Embed(v.F)
	if err != nil {
		return nil, err
	}
	inner["F"] = fObj

	if len(v.MainImage) > 0 {
		inner["MainImage"] = v.MainImage
	}
	if len(v.Tags) > 0 {
		arr := make(pdf.Array, 0, 2*len(v.Tags))
		for _, t := range v.Tags {
			arr = append(arr, pdf.Integer(t.Num))
			if len(t.Text) == 1 {
				arr = append(arr, pdf.TextString(t.Text[0]))
			} else {
				texts := make(pdf.Array, len(t.Text))
				for i, s := range t.Text {
					texts[i] = pdf.TextString(s)
				}
				arr = append(arr, texts)
			}
		}
		inner["Tags"] = arr
	}
	if v.Size != nil {
		inner["Size"] = numbersToArray(v.Size[:])
	}
	if v.CropRect != nil {
		inner["CropRect"] = numbersToArray(v.CropRect[:])
	}
	if v.Overprint {
		inner["Overprint"] = pdf.Boolean(true)
	}
	if v.Inks != nil {
		if v.Inks.Name != "" {
			inner["Inks"] = v.Inks.Name
		} else if len(v.Inks.Monochrome) > 0 {
			arr := make(pdf.Array, 0, 1+2*len(v.Inks.Monochrome))
			arr = append(arr, pdf.Name("monochrome"))
			for _, c := range v.Inks.Monochrome {
				arr = append(arr, c.Name, pdf.Number(c.Tint))
			}
			inner["Inks"] = arr
		}
	}
	if v.IncludedImageDimensions != nil {
		inner["IncludedImageDimensions"] = intsToArray(v.IncludedImageDimensions[:])
	}
	if v.IncludedImageQuality != 0 {
		inner["IncludedImageQuality"] = pdf.Number(v.IncludedImageQuality)
	}

	return embedVersion(rm, "2.0", inner, v.SingleUse)
}

func extractV20(x *pdf.Extractor, path *pdf.CycleCheck, obj pdf.Object, isDirect bool) (*V20, error) {
	dict, err := x.GetDictTyped(path, obj, "OPI")
	if err != nil {
		return nil, err
	} else if dict == nil {
		return nil, pdf.Error("missing OPI dictionary")
	}

	v := &V20{SingleUse: isDirect}

	fs, err := pdf.ExtractorGet(x, path, dict["F"], file.ExtractSpecification)
	if err != nil {
		return nil, err
	} else if fs == nil {
		return nil, pdf.Error("OPI dictionary requires F entry")
	}
	v.F = fs

	if mi, err := pdf.Optional(x.GetString(path, dict["MainImage"])); err != nil {
		return nil, err
	} else if len(mi) > 0 {
		v.MainImage = mi
	}
	if tags, err := pdf.Optional(x.GetArray(path, dict["Tags"])); err != nil {
		return nil, err
	} else if len(tags) >= 2 {
		for i := 0; i+1 < len(tags); i += 2 {
			num, err := pdf.Optional(x.GetInteger(path, tags[i]))
			if err != nil {
				return nil, err
			}
			text, err := readTagText(x, path, tags[i+1])
			if err != nil {
				return nil, err
			}
			v.Tags = append(v.Tags, Tag20{Num: int(num), Text: text})
		}
	}
	if s, err := readNumbers(x, path, dict["Size"]); err != nil {
		return nil, err
	} else if len(s) == 2 {
		v.Size = &[2]float64{s[0], s[1]}
	}
	if c, err := readNumbers(x, path, dict["CropRect"]); err != nil {
		return nil, err
	} else if len(c) == 4 {
		v.CropRect = &[4]float64{c[0], c[1], c[2], c[3]}
	}
	if o, err := pdf.Optional(x.GetBoolean(path, dict["Overprint"])); err != nil {
		return nil, err
	} else {
		v.Overprint = bool(o)
	}
	if inks, err := readInks(x, path, dict["Inks"]); err != nil {
		return nil, err
	} else {
		v.Inks = inks
	}
	if d, err := readInts(x, path, dict["IncludedImageDimensions"]); err != nil {
		return nil, err
	} else if len(d) == 2 {
		v.IncludedImageDimensions = &[2]int{d[0], d[1]}
	}
	if q, err := pdf.Optional(x.GetNumber(path, dict["IncludedImageQuality"])); err != nil {
		return nil, err
	} else {
		v.IncludedImageQuality = float64(q)
	}

	// Size and CropRect are present or absent together.
	if (v.Size == nil) != (v.CropRect == nil) {
		v.Size = nil
		v.CropRect = nil
	}
	// valid IncludedImageQuality values are 1, 2, and 3; treat anything else
	// as absent
	if q := v.IncludedImageQuality; q != 1 && q != 2 && q != 3 {
		v.IncludedImageQuality = 0
	}

	return v, nil
}

// readTagText reads an OPI 2.0 tag value, which is a single ASCII string or an
// array of ASCII strings.
func readTagText(x *pdf.Extractor, path *pdf.CycleCheck, obj pdf.Object) ([]string, error) {
	resolved, err := pdf.Resolve(x.R, obj)
	if err != nil {
		return nil, err
	}
	switch t := resolved.(type) {
	case pdf.Array:
		if len(t) == 0 {
			return nil, nil
		}
		out := make([]string, 0, len(t))
		for _, el := range t {
			s, err := pdf.Optional(x.GetString(path, el))
			if err != nil {
				return nil, err
			}
			out = append(out, string(s.AsTextString()))
		}
		return out, nil
	case pdf.String:
		return []string{string(t.AsTextString())}, nil
	default:
		return nil, nil
	}
}

// readInks reads the OPI 2.0 Inks entry, which is either a name or a
// /monochrome array.
func readInks(x *pdf.Extractor, path *pdf.CycleCheck, obj pdf.Object) (*Inks20, error) {
	resolved, err := pdf.Resolve(x.R, obj)
	if err != nil {
		return nil, err
	}
	switch t := resolved.(type) {
	case pdf.Name:
		return &Inks20{Name: t}, nil
	case pdf.Array:
		inks := &Inks20{}
		for i := 1; i+1 < len(t); i += 2 {
			name, err := pdf.Optional(x.GetString(path, t[i]))
			if err != nil {
				return nil, err
			}
			tint, err := pdf.Optional(x.GetNumber(path, t[i+1]))
			if err != nil {
				return nil, err
			}
			inks.Monochrome = append(inks.Monochrome, Ink20Comp{Name: name, Tint: float64(tint)})
		}
		if len(inks.Monochrome) == 0 {
			return nil, nil
		}
		return inks, nil
	default:
		return nil, nil
	}
}
