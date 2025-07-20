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

	"golang.org/x/text/language"
	"seehuhn.de/go/pdf"
)

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
	// freeform string.
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
	Appearance *AppearanceDict

	// AppearanceState (required if AP contains subdictionaries) is the
	// annotation's appearance state, which selects the applicable appearance
	// stream from an appearance subdictionary.
	//
	// This corresponds to the /AS entry in the PDF annotation dictionary.
	AppearanceState pdf.Name

	// Border (optional) specifies the characteristics of the annotation's
	// border.
	Border *Border

	// Color (optional) is an array of numbers representing a color used for
	// the annotation's background, title bar or border.
	//
	// This corresponds to the /C entry in the PDF annotation dictionary.
	Color []float64

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

	// NonStrokingOpacity is the opacity value for all nonstroking operations
	// on all visible elements of the annotation in its closed state.
	// The value 0.0 means fully transparent, 1.0 means fully opaque.
	// Ignored if the annotation has an appearance stream.
	// For PDF versions prior to 2.0, this field must equal StrokingOpacity.
	NonStrokingOpacity float64

	// StrokingOpacity is the opacity value for stroking all visible elements
	// of the annotation in its closed state. The value 0.0 means fully
	// transparent, 1.0 means fully opaque. Ignored if the annotation has an
	// appearance stream. For non-markup annotations prior to PDF 2.0, this
	// field must be 1.0.
	StrokingOpacity float64

	// BlendMode (optional) is the blend mode that is used when painting the
	// annotation onto the page.
	//
	// This corresponds to the /BM entry in the PDF annotation dictionary.
	BlendMode pdf.Name

	// Lang (optional) is a language identifier specifying the natural language
	// for all text in the annotation.
	Lang language.Tag
}

// fillDict adds the fields corresponding to the Common struct
// to the given PDF dictionary.  If fields are not valid for the PDF version
// corresponding to the ResourceManager, an error is returned.
func (c *Common) fillDict(rm *pdf.ResourceManager, d pdf.Dict) error {
	w := rm.Out

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
	}

	if c.AppearanceState != "" {
		if err := pdf.CheckVersion(w, "annotation AS entry", pdf.V1_2); err != nil {
			return err
		}
		d["AS"] = c.AppearanceState
	} else if c.Appearance.hasDicts() {
		return errors.New("missing AS entry")
	}

	if c.Border != nil && !c.Border.isDefault() {
		borderArray := pdf.Array{
			pdf.Number(c.Border.HCornerRadius),
			pdf.Number(c.Border.VCornerRadius),
			pdf.Number(c.Border.Width),
		}
		if c.Border.DashArray != nil {
			if err := pdf.CheckVersion(w, "annotation Border dash array", pdf.V1_1); err != nil {
				return err
			}
			dashArray := make(pdf.Array, len(c.Border.DashArray))
			for i, v := range c.Border.DashArray {
				dashArray[i] = pdf.Number(v)
			}
			borderArray = append(borderArray, dashArray)
		}
		d["Border"] = borderArray
	}

	if c.Color != nil {
		if err := pdf.CheckVersion(w, "annotation C entry", pdf.V1_1); err != nil {
			return err
		}
		colorArray := make(pdf.Array, len(c.Color))
		for i, v := range c.Color {
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
	if c.StrokingOpacity != 1.0 {
		if err := pdf.CheckVersion(w, "annotation CA entry", pdf.V1_4); err != nil {
			return err
		}
		d["CA"] = pdf.Number(c.StrokingOpacity)
	}

	// NonStrokingOpacity (ca entry)
	if c.NonStrokingOpacity != c.StrokingOpacity {
		if err := pdf.CheckVersion(w, "annotation ca entry", pdf.V2_0); err != nil {
			return err
		}
		d["ca"] = pdf.Number(c.NonStrokingOpacity)
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

// extractCommon extracts fields common to all annotations from a PDF dictionary.
func extractCommon(r pdf.Getter, dict pdf.Dict, common *Common) error {
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
	if ap, err := ExtractAppearanceDict(r, dict["AP"]); err == nil && ap != nil {
		common.Appearance = ap
	}

	// AS (optional)
	if as, err := pdf.GetName(r, dict["AS"]); err == nil && as != "" {
		common.AppearanceState = as
	}

	// Border (optional)
	if border, err := pdf.GetArray(r, dict["Border"]); err == nil && border != nil {
		if len(border) >= 3 {
			b := &Border{}
			if h, err := pdf.GetNumber(r, border[0]); err == nil {
				b.HCornerRadius = float64(h)
			}
			if v, err := pdf.GetNumber(r, border[1]); err == nil {
				b.VCornerRadius = float64(v)
			}
			if w, err := pdf.GetNumber(r, border[2]); err == nil {
				b.Width = float64(w)
			}
			if len(border) > 3 {
				if dashArray, err := pdf.GetArray(r, border[3]); err == nil {
					dashes := make([]float64, len(dashArray))
					for i, dash := range dashArray {
						if num, err := pdf.GetNumber(r, dash); err == nil {
							dashes[i] = float64(num)
						}
					}
					b.DashArray = dashes
				}
			}
			common.Border = b
		}
	}

	// C (optional)
	if c, err := pdf.GetArray(r, dict["C"]); err == nil && len(c) > 0 {
		colors := make([]float64, len(c))
		for i, color := range c {
			if num, err := pdf.GetNumber(r, color); err == nil {
				colors[i] = float64(num)
			}
		}
		common.Color = colors
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
			common.StrokingOpacity = float64(ca)
		}
	} else {
		common.StrokingOpacity = 1.0
	}

	// ca (optional) - if not present, defaults to the same value as CA
	if dict["ca"] != nil {
		if ca, err := pdf.GetNumber(r, dict["ca"]); err == nil {
			common.NonStrokingOpacity = float64(ca)
		}
	} else {
		common.NonStrokingOpacity = common.StrokingOpacity
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
