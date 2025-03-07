package form

import (
	"bytes"
	"time"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
)

type Form struct {
	Draw         func(*graphics.Writer) error
	BBox         pdf.Rectangle
	Matrix       matrix.Matrix
	Metadata     pdf.Reference
	PieceInfo    pdf.Object
	LastModified time.Time
	// TODO(voss): StructParent, StructParents
	OC      pdf.Object
	AF      pdf.Object
	Measure pdf.Object
	PtData  pdf.Object
}

func (f *Form) Subtype() pdf.Name {
	return "Form"
}

func (f *Form) validate() error {
	if f.BBox.IsZero() {
		return pdf.Error("missing BBox")
	}
	return nil
}

func (f *Form) Embed(rm *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
	err := f.validate()
	if err != nil {
		return nil, pdf.Unused{}, err
	}

	buf := &bytes.Buffer{}
	contents := graphics.NewWriter(buf, rm)
	err = f.Draw(contents)
	if err != nil {
		return nil, pdf.Unused{}, err
	}

	ref := rm.Out.Alloc()

	dict := pdf.Dict{
		"Subtype": pdf.Name("Form"),
		"BBox":    &f.BBox,
	}
	if rm.Out.GetOptions().HasAny(pdf.OptDictTypes) {
		dict["Type"] = pdf.Name("XObject")
	}
	if f.Matrix != matrix.Identity && f.Matrix != matrix.Zero {
		dict["Matrix"] = toPDF(f.Matrix[:])
	}
	if contents.Resources != nil {
		dict["Resources"] = pdf.AsDict(contents.Resources)
	}
	if f.Metadata != 0 {
		dict["Metadata"] = f.Metadata
	}
	if f.PieceInfo != nil {
		dict["PieceInfo"] = f.PieceInfo
	}
	if !f.LastModified.IsZero() {
		dict["LastModified"] = pdf.Date(f.LastModified)
	}
	if f.OC != nil {
		dict["OC"] = f.OC
	}
	if f.AF != nil {
		dict["AF"] = f.AF
	}
	if f.Measure != nil {
		dict["Measure"] = f.Measure
	}
	if f.PtData != nil {
		dict["PtData"] = f.PtData
	}

	stm, err := rm.Out.OpenStream(ref, dict, &pdf.FilterCompress{})
	if err != nil {
		return nil, pdf.Unused{}, err
	}
	_, err = stm.Write(buf.Bytes())
	if err != nil {
		return nil, pdf.Unused{}, err
	}
	err = stm.Close()
	if err != nil {
		return nil, pdf.Unused{}, err
	}

	return ref, pdf.Unused{}, nil
}

func toPDF(x []float64) pdf.Array {
	res := make(pdf.Array, len(x))
	for i, xi := range x {
		res[i] = pdf.Number(xi)
	}
	return res
}
