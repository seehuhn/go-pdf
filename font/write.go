package font

import "seehuhn.de/go/pdf"

type XFont struct {
	// One of "Type1", "TrueType" or "Type3".
	SubType string

	// The name by which this font is referenced in the Font subdictionary of
	// the current resource dictionary.
	Name string

	// The PostScript name of the font.  For Type 1 fonts, this is the value of
	// the FontName entry in the font program. For TrueType fonts, if the
	// "name" table contains a PostScript name, this should be used. Otherwise,
	// a PostScript name shall be derived from the name by which the font is
	// known in the host operating system (page 257).
	BaseFont string

	// TODO(voss): derive this from `BaseFont`?
	// The PostScript names of standard fonts are: Times-Roman, Helvetica,
	// Courier, Symbol, Times-Bold, Helvetica-Bold, Courier-Bold, ZapfDingbats,
	// Times-Italic, Helvetica-Oblique, Courier-Oblique, Times-BoldItalic,
	// Helvetica-BoldOblique, Courier-BoldOblique
	IsStandard bool

	FirstChar int
	LastChar  int
	Widths    []float64 // TODO(voss): are these integers?

	Encoding Encoding
}

func (font *XFont) writeWidths(w pdf.Writer) (*pdf.Reference, error) {
	var width pdf.Array
	for _, w := range font.Widths {
		wi := pdf.Integer(w)
		if float64(wi) == w {
			width = append(width, wi)
		} else {
			width = append(width, pdf.Real(w))
		}
	}
	return w.Write(width, nil)
}

func WriteXFont(w pdf.Writer, font *XFont) (*pdf.Reference, error) {
	dict := pdf.Dict{
		"Type":    pdf.Name("Font"),
		"Subtype": pdf.Name(font.SubType),
	}
	if w.Version == pdf.V1_0 {
		dict["Name"] = pdf.Name(font.Name)
	}

	if font.Encoding != nil {
		dict["Encoding"] = Describe(font.Encoding)
	}

	switch font.SubType {
	case "Type1", "TrueType":
		dict["BaseFont"] = pdf.Name(font.BaseFont)

		var FontDescriptor *pdf.Reference
		if !font.IsStandard || w.Version >= pdf.V1_5 {
			// set FontDescriptor
		}

		if FontDescriptor != nil {
			widthRef, err := font.writeWidths(w)
			if err != nil {
				return nil, err
			}

			dict["FirstChar"] = pdf.Integer(font.FirstChar)
			dict["LastChar"] = pdf.Integer(font.LastChar)
			dict["Widths"] = widthRef
			dict["FontDescriptor"] = FontDescriptor
		}

		// TODO(voss): there is also "ToUnicode"
		// TODO(voss): add support for font subsets

	case "Type3":
		widthRef, err := font.writeWidths(w)
		if err != nil {
			return nil, err
		}

		dict["FirstChar"] = pdf.Integer(font.FirstChar)
		dict["LastChar"] = pdf.Integer(font.LastChar)
		dict["Widths"] = widthRef
		// TODO(voss): "FontDescriptor" is required in Tagged PDF documents.

		panic("not implemented")
	}

	return w.Write(dict, nil)
}
