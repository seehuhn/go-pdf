package annotation

import (
	"errors"

	"golang.org/x/text/language"
	"seehuhn.de/go/pdf"
)

// Common contains fields common to all annotation dictionaries.
type Common struct {
	// Rect (required) is the rectangle that defines the position of the
	// annotation on the page (in user space coordinates).
	Rect pdf.Rectangle

	// Contents (optional) is the textual content of the annotation.
	// The exact meaning of this field depends on the type of the annotation.
	Contents string

	// P (optional; PDF 1.3; not used in FDF files) points to the page
	// dictionary that contains this annotation. Required for screen
	// annotations associated with rendition actions.
	P pdf.Reference

	// NM (optional; PDF 1.4) is the annotation name, a text string uniquely
	// identifying it among all the annotations on its page.
	NM string

	// M (optional; PDF 1.1) is the date and time when the annotation was
	// most recently modified.  This is either a PDF date string or a
	// freeform string.
	M string

	// F (optional; PDF 1.1) is a set of flags specifying various
	// characteristics of the annotation. Default value: 0.
	F pdf.Integer

	// AP (optional; PDF 1.2) is an appearance dictionary specifying how the
	// annotation shall be presented visually on the page.
	AP *AppearanceDict

	// AS (required if AP contains subdictionaries; PDF 1.2) is the
	// annotation's appearance state, which selects the applicable appearance
	// stream from an appearance subdictionary.
	AS pdf.Name

	// Border (optional) specifies the characteristics of the annotation's
	// border. Default value: [0 0 1].
	Border *Border

	// C (optional; PDF 1.1) is an array of numbers representing a color used
	// for the annotation's background, title bar, or border.
	C []float64

	// StructParent (required if the annotation is a structural content item;
	// PDF 1.3) is the integer key of the annotation's entry in the structural
	// parent tree.
	StructParent pdf.Integer

	// OC (optional; PDF 1.5) specifies the optional content properties for
	// the annotation.
	OC pdf.Reference

	// AF (optional; PDF 2.0) is an array of file specification dictionaries
	// which denote the associated files for this annotation.
	AF []pdf.Reference

	// NonStrokingOpacity is the opacity value for all nonstroking operations
	// on all visible elements of the annotation in its closed state.
	// The value 0.0 means fully transparent, 1.0 means fully opaque.
	// Ignored if the annotation has an appearance stream.
	// For PDF versions prior to 2.0, this field must equal StrokingOpacity.
	NonStrokingOpacity float64

	// StrokingOpacity is the opacity value for stroking all visible elements
	// of the annotation in its closed state.
	// The value 0.0 means fully transparent, 1.0 means fully opaque.
	// Ignored if the annotation has an appearance stream.
	// For PDF versions prior to 1.4, and for non-markup annotations prior to
	// PDF 2.0, this field must be 1.0.
	StrokingOpacity float64

	// BM (optional; PDF 2.0) is the blend mode that shall be used when
	// painting the annotation onto the page.
	BM pdf.Name

	// Lang (optional; PDF 2.0) is a language identifier specifying the
	// natural language for all text in the annotation.
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

	if c.P != 0 {
		if err := pdf.CheckVersion(w, "annotation P entry", pdf.V1_3); err != nil {
			return err
		}
		d["P"] = c.P
	}

	if c.NM != "" {
		if err := pdf.CheckVersion(w, "annotation NM entry", pdf.V1_4); err != nil {
			return err
		}
		d["NM"] = pdf.TextString(c.NM)
	}

	if c.M != "" {
		if err := pdf.CheckVersion(w, "annotation M entry", pdf.V1_1); err != nil {
			return err
		}
		d["M"] = pdf.TextString(c.M)
	}

	if c.F != 0 {
		if err := pdf.CheckVersion(w, "annotation F entry", pdf.V1_1); err != nil {
			return err
		}
		d["F"] = c.F
	}

	if c.AP != nil {
		if err := pdf.CheckVersion(w, "annotation AP entry", pdf.V1_2); err != nil {
			return err
		}
		ref, _, err := pdf.ResourceManagerEmbed(rm, c.AP)
		if err != nil {
			return err
		}
		d["AP"] = ref
	}

	if c.AS != "" {
		if err := pdf.CheckVersion(w, "annotation AS entry", pdf.V1_2); err != nil {
			return err
		}
		d["AS"] = c.AS
	} else if c.AP.hasDicts() {
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

	if c.C != nil {
		if err := pdf.CheckVersion(w, "annotation C entry", pdf.V1_1); err != nil {
			return err
		}
		colorArray := make(pdf.Array, len(c.C))
		for i, v := range c.C {
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

	if c.OC != 0 {
		if err := pdf.CheckVersion(w, "annotation OC entry", pdf.V1_5); err != nil {
			return err
		}
		d["OC"] = c.OC
	}

	if c.AF != nil {
		if err := pdf.CheckVersion(w, "annotation AF entry", pdf.V2_0); err != nil {
			return err
		}
		afArray := make(pdf.Array, len(c.AF))
		for i, ref := range c.AF {
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

	if c.BM != "" {
		if err := pdf.CheckVersion(w, "annotation BM entry", pdf.V2_0); err != nil {
			return err
		}
		d["BM"] = c.BM
	}

	if !c.Lang.IsRoot() {
		if err := pdf.CheckVersion(w, "annotation Lang entry", pdf.V2_0); err != nil {
			return err
		}
		d["Lang"] = pdf.TextString(c.Lang.String())
	}

	return nil
}

// Border represents the characteristics of an annotation's border.
type Border struct {
	// HCornerRadius is the horizontal corner radius.
	HCornerRadius float64

	// VCornerRadius is the vertical corner radius.
	VCornerRadius float64

	// Width is the border width in default user space units.
	// If 0, no border is drawn.
	Width float64

	// DashArray (optional; PDF 1.1) defines a pattern of dashes and gaps
	// for drawing the border. If nil, a solid border is drawn.
	DashArray []float64
}

func (b *Border) isDefault() bool {
	return b.HCornerRadius == 0 && b.VCornerRadius == 0 && b.Width == 1 && b.DashArray == nil
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
		common.P = p
	}

	// NM (optional)
	if nm, err := pdf.GetTextString(r, dict["NM"]); err == nil && nm != "" {
		common.NM = string(nm)
	}

	// M (optional)
	if m, err := pdf.GetTextString(r, dict["M"]); err == nil && m != "" {
		common.M = string(m)
	}

	// F (optional)
	if f, err := pdf.GetInteger(r, dict["F"]); err == nil && f != 0 {
		common.F = f
	}

	// AP (optional)
	if ap, ok := dict["AP"].(pdf.Reference); ok {
		// TODO: implement appearance dictionary extraction
		_ = ap
	}

	// AS (optional)
	if as, err := pdf.GetName(r, dict["AS"]); err == nil && as != "" {
		common.AS = as
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
		common.C = colors
	}

	// StructParent (optional)
	if sp, err := pdf.GetInteger(r, dict["StructParent"]); err == nil && sp != 0 {
		common.StructParent = sp
	}

	// OC (optional)
	if oc, ok := dict["OC"].(pdf.Reference); ok {
		common.OC = oc
	}

	// AF (optional)
	if af, err := pdf.GetArray(r, dict["AF"]); err == nil && len(af) > 0 {
		refs := make([]pdf.Reference, len(af))
		for i, fileRef := range af {
			if ref, ok := fileRef.(pdf.Reference); ok {
				refs[i] = ref
			}
		}
		common.AF = refs
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
		common.BM = bm
	}

	// Lang (optional)
	if lang, err := pdf.GetTextString(r, dict["Lang"]); err == nil && lang != "" {
		if tag, err := language.Parse(string(lang)); err == nil {
			common.Lang = tag
		}
	}

	return nil
}
