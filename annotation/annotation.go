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

package annotation

import (
	"errors"
	"fmt"

	"golang.org/x/text/language"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation/appearance"
	"seehuhn.de/go/pdf/graphics/color"
)

// Annotation represents a PDF annotation.
type Annotation interface {
	pdf.Encoder

	// AnnotationType returns the type of the annotation, e.g. "Text", "Link",
	// "Widget", etc.
	AnnotationType() pdf.Name

	// GetCommon returns the common annotation fields.
	GetCommon() *Common
}

// Common contains fields common to all annotation dictionaries.
type Common struct {
	// Rect is the rectangle that defines the position and sometimes the extent
	// of the annotation on the page (in default user space coordinates).
	// The position is given by the upper-left corner of the rectangle.
	Rect pdf.Rectangle

	// Contents (optional) is the textual content of the annotation.
	// The exact meaning of this field depends on the type of the annotation.
	Contents string

	// Page (optional) points to the page dictionary that contains this
	// annotation. Required for [Screen] annotations associated with rendition
	// actions.
	//
	// This corresponds to the /P entry in the PDF annotation dictionary.
	Page pdf.Reference

	// Name (optional) is the name of the annotation. Is set, this must be unique
	// among all annotations on the page.
	//
	// This corresponds to the /NM entry in the PDF annotation dictionary.
	Name string

	// LastModified (optional) is the date and time when the annotation was
	// most recently modified.  This can either be a PDF date string or a
	// human-readable freeform string.
	//
	// This corresponds to the /M entry in the PDF annotation dictionary.
	LastModified string

	// Flags specifies various characteristics of the annotation.
	//
	// This corresponds to the /F entry in the PDF annotation dictionary.
	Flags Flags

	// Appearance (optional) is an appearance dictionary specifying how the
	// annotation is presented visually on the page.
	//
	// This corresponds to the /AP entry in the PDF annotation dictionary.
	Appearance *appearance.Dict

	// AppearanceState (required if AP contains subdictionaries) is the
	// annotation's appearance state, which selects the applicable appearance
	// stream from an appearance subdictionary.
	//
	// This corresponds to the /AS entry in the PDF annotation dictionary.
	AppearanceState pdf.Name

	// Border (optional) specifies the characteristics of the annotation's
	// border.
	//
	// When writing annotations, a nil value can be used as a shorthand for
	// the default border style (width 1, solid line, no dash or rounded corners).
	Border *Border

	// Color (optional) is the color used for the annotation's background,
	// title bar or border (depending on the annotation type).
	//
	// Only certain color types are allowed:
	//  - colors in the [color.DeviceGray] color space
	//  - colors in the [color.DeviceRGB] color space
	//  - colors in the [color.DeviceCMYK] color space
	//  - the [Transparent] color
	//
	// This corresponds to the /C entry in the PDF annotation dictionary.
	Color color.Color

	// StructParent (required if the annotation is a structural content item)
	// is the integer key of the annotation's entry in the structural parent
	// tree.
	StructParent pdf.Integer

	// OptionalContent (optional) specifies the optional content properties for
	// the annotation.
	//
	// This corresponds to the /OC entry in the PDF annotation dictionary.
	OptionalContent pdf.Reference

	// Files (optional) is an array of file specification dictionaries which
	// denote the associated files for this annotation.
	//
	// This corresponds to the /AF entry in the PDF annotation dictionary.
	Files []pdf.Reference

	// NonStrokingTransparency is the transparency value for nonstroking
	// operations on the annotation in its closed state. The value 1 means
	// fully transparent, 0 means fully opaque. Ignored if the annotation has
	// an appearance stream.  For PDF versions prior to 2.0, this field must
	// equal StrokingTransparency.
	//
	// This represents the /ca entry in the PDF annotation dictionary (ca = 1 -
	// NonStrokingTransparency).
	NonStrokingTransparency float64

	// StrokingTransparency is the transparency value for stroking operations
	// on annotation in its closed state. The value 1 means fully transparent,
	// 0 means fully opaque. Ignored if the annotation has an appearance
	// stream.  For non-markup annotations prior to PDF 2.0, this field must be
	// 0.
	//
	// This represents the /CA entry in the PDF annotation dictionary. (CA = 1
	// - StrokingTransparency).
	StrokingTransparency float64

	// BlendMode (optional) is the blend mode that is used when painting the
	// annotation onto the page.
	//
	// This corresponds to the /BM entry in the PDF annotation dictionary.
	BlendMode pdf.Name

	// Lang (optional) is a language identifier specifying the natural language
	// for all text in the annotation.
	Lang language.Tag
}

func (c *Common) GetCommon() *Common {
	return c
}

// fillDict adds the fields corresponding to the Common struct
// to the given PDF dictionary.  If fields are not valid for the PDF version
// corresponding to the ResourceManager, an error is returned.
func (c *Common) fillDict(rm *pdf.ResourceManager, d pdf.Dict, isMarkup bool) error {
	w := rm.Out

	if rm.Out.GetOptions().HasAny(pdf.OptDictTypes) {
		d["Type"] = pdf.Name("Annot")
	}

	d["Rect"] = &c.Rect

	if c.Contents != "" {
		d["Contents"] = pdf.TextString(c.Contents)
	}

	if c.Page != 0 {
		if err := pdf.CheckVersion(w, "annotation P entry", pdf.V1_3); err != nil {
			return err
		}
		d["P"] = c.Page
	}

	if c.Name != "" {
		if err := pdf.CheckVersion(w, "annotation NM entry", pdf.V1_4); err != nil {
			return err
		}
		d["NM"] = pdf.TextString(c.Name)
	}

	if c.LastModified != "" {
		if err := pdf.CheckVersion(w, "annotation M entry", pdf.V1_1); err != nil {
			return err
		}
		d["M"] = pdf.TextString(c.LastModified)
	}

	if c.Flags != 0 {
		if err := pdf.CheckVersion(w, "annotation F entry", pdf.V1_1); err != nil {
			return err
		}
		d["F"] = pdf.Integer(c.Flags)
	}

	if c.Appearance != nil {
		if err := pdf.CheckVersion(w, "annotation AP entry", pdf.V1_2); err != nil {
			return err
		}
		ref, _, err := pdf.ResourceManagerEmbed(rm, c.Appearance)
		if err != nil {
			return err
		}
		d["AP"] = ref
	} else {
		// check for missing appearance dictionary per PDF spec requirements
		subtype := d["Subtype"].(pdf.Name)
		isSinglePoint := c.Rect.LLx == c.Rect.URx && c.Rect.LLy == c.Rect.URy
		isExemptSubtype := subtype == "Popup" || subtype == "Projection" || subtype == "Link"
		if pdf.GetVersion(w) >= pdf.V2_0 && !isSinglePoint && !isExemptSubtype {
			return errors.New("missing appearance dictionary")
		}
	}

	if c.AppearanceState != "" {
		if err := pdf.CheckVersion(w, "annotation AS entry", pdf.V1_2); err != nil {
			return err
		}
		d["AS"] = c.AppearanceState
	} else if c.Appearance != nil && c.Appearance.HasDicts() {
		return errors.New("missing AS entry")
	}

	if c.Border != nil {
		borderValue, _, err := pdf.ResourceManagerEmbed(rm, c.Border)
		if err != nil {
			return err
		}
		if borderValue != nil {
			d["Border"] = borderValue
		}
	}

	if c.Color != nil {
		if err := pdf.CheckVersion(w, "annotation C entry", pdf.V1_1); err != nil {
			return err
		}
		s := c.Color.ColorSpace()
		var x []float64
		if s != nil {
			fam := s.Family()
			switch fam {
			case color.FamilyDeviceGray, color.FamilyDeviceRGB, color.FamilyDeviceCMYK:
				x, _, _ = color.Operator(c.Color)
			default:
				return fmt.Errorf("unexpected color space %s", fam)
			}
		}
		colorArray := make(pdf.Array, len(x))
		for i, v := range x {
			colorArray[i] = pdf.Number(v)
		}
		d["C"] = colorArray
	}

	if c.StructParent != 0 {
		if err := pdf.CheckVersion(w, "annotation StructParent entry", pdf.V1_3); err != nil {
			return err
		}
		d["StructParent"] = c.StructParent
	}

	if c.OptionalContent != 0 {
		if err := pdf.CheckVersion(w, "annotation OC entry", pdf.V1_5); err != nil {
			return err
		}
		d["OC"] = c.OptionalContent
	}

	if c.Files != nil {
		if err := pdf.CheckVersion(w, "annotation AF entry", pdf.V2_0); err != nil {
			return err
		}
		afArray := make(pdf.Array, len(c.Files))
		for i, ref := range c.Files {
			afArray[i] = ref
		}
		d["AF"] = afArray
	}

	// StrokingOpacity (CA entry)
	if c.StrokingTransparency != 0.0 {
		if isMarkup {
			if err := pdf.CheckVersion(w, "markup annotation CA entry", pdf.V1_4); err != nil {
				return err
			}
		} else {
			if err := pdf.CheckVersion(w, "non-markup annotation CA entry", pdf.V2_0); err != nil {
				return err
			}
		}
		d["CA"] = pdf.Number(1 - c.StrokingTransparency)
	}

	// NonStrokingOpacity (ca entry)
	if c.NonStrokingTransparency != c.StrokingTransparency {
		if err := pdf.CheckVersion(w, "annotation ca entry", pdf.V2_0); err != nil {
			return err
		}
		d["ca"] = pdf.Number(1 - c.NonStrokingTransparency)
	}

	if c.BlendMode != "" {
		if err := pdf.CheckVersion(w, "annotation BM entry", pdf.V2_0); err != nil {
			return err
		}
		d["BM"] = c.BlendMode
	}

	if !c.Lang.IsRoot() {
		if err := pdf.CheckVersion(w, "annotation Lang entry", pdf.V2_0); err != nil {
			return err
		}
		d["Lang"] = pdf.TextString(c.Lang.String())
	}

	return nil
}

// decodeCommon extracts fields common to all annotations from a PDF dictionary.
func decodeCommon(r pdf.Getter, common *Common, dict pdf.Dict) error {
	// Rect (required)
	if rect, err := pdf.GetRectangle(r, dict["Rect"]); err == nil && rect != nil {
		common.Rect = *rect
	}

	// Contents (optional)
	if contents, err := pdf.GetTextString(r, dict["Contents"]); err == nil && contents != "" {
		common.Contents = string(contents)
	}

	// P (optional)
	if p, ok := dict["P"].(pdf.Reference); ok {
		common.Page = p
	}

	// NM (optional)
	if nm, err := pdf.GetTextString(r, dict["NM"]); err == nil && nm != "" {
		common.Name = string(nm)
	}

	// M (optional)
	if m, err := pdf.GetTextString(r, dict["M"]); err == nil && m != "" {
		common.LastModified = string(m)
	}

	// F (optional)
	if f, err := pdf.GetInteger(r, dict["F"]); err == nil && f != 0 {
		common.Flags = Flags(f)
	}

	// AP (optional)
	if ap, err := appearance.Extract(r, dict["AP"]); err == nil && ap != nil {
		common.Appearance = ap
	}

	// AS (optional)
	if as, err := pdf.GetName(r, dict["AS"]); err == nil && as != "" {
		common.AppearanceState = as
	}

	// Border (optional)
	if dict["Border"] != nil {
		if border, err := ExtractBorder(r, dict["Border"]); err == nil {
			common.Border = border
		}
	}

	// C (optional)
	if c, _ := pdf.GetArray(r, dict["C"]); c != nil {
		colors := make([]float64, len(c))
		for i, color := range c {
			if num, err := pdf.GetNumber(r, color); err == nil {
				colors[i] = float64(num)
			}
		}
		switch len(colors) {
		case 0:
			common.Color = Transparent
		case 1:
			common.Color = color.DeviceGray(colors[0])
		case 3:
			common.Color = color.DeviceRGB(colors[0], colors[1], colors[2])
		case 4:
			common.Color = color.DeviceCMYK(colors[0], colors[1], colors[2], colors[3])
		}
	}

	// StructParent (optional)
	if sp, err := pdf.GetInteger(r, dict["StructParent"]); err == nil && sp != 0 {
		common.StructParent = sp
	}

	// OC (optional)
	if oc, ok := dict["OC"].(pdf.Reference); ok {
		common.OptionalContent = oc
	}

	// AF (optional)
	if af, err := pdf.GetArray(r, dict["AF"]); err == nil && len(af) > 0 {
		refs := make([]pdf.Reference, len(af))
		for i, fileRef := range af {
			if ref, ok := fileRef.(pdf.Reference); ok {
				refs[i] = ref
			}
		}
		common.Files = refs
	}

	// CA (optional) - default value is 1.0
	if dict["CA"] != nil {
		if ca, err := pdf.GetNumber(r, dict["CA"]); err == nil {
			common.StrokingTransparency = 1 - float64(ca)
		}
	}

	// ca (optional) - if not present, defaults to the same value as CA
	if dict["ca"] != nil {
		if ca, err := pdf.GetNumber(r, dict["ca"]); err == nil {
			common.NonStrokingTransparency = 1 - float64(ca)
		}
	} else {
		common.NonStrokingTransparency = common.StrokingTransparency
	}

	// BM (optional)
	if bm, err := pdf.GetName(r, dict["BM"]); err == nil && bm != "" {
		common.BlendMode = bm
	}

	// Lang (optional)
	if lang, err := pdf.GetTextString(r, dict["Lang"]); err == nil && lang != "" {
		if tag, err := language.Parse(string(lang)); err == nil {
			common.Lang = tag
		}
	}

	return nil
}

// Transparent is a special color that indicates that an annotation should not
// be painted at all.  This can only be used for the Color field in the
// [Common] struct.
var Transparent color.Color = &transparent{}

type transparent struct{}

func (t *transparent) ColorSpace() color.Space {
	return nil
}
