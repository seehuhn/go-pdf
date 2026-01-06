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

// Package page implements PDF page objects.
//
// PDF 2.0 sections: 7.7.3
package page

import (
	"time"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/extract"
	"seehuhn.de/go/pdf/graphics/image/thumbnail"
	"seehuhn.de/go/pdf/metadata"
	"seehuhn.de/go/pdf/optional"
	"seehuhn.de/go/pdf/pieceinfo"
)

// PageContent represents a single content stream that can be shared
// across multiple pages. It implements [pdf.Embedder] for deduplication.
type PageContent struct {
	Operators content.Stream
}

var _ pdf.Embedder = (*PageContent)(nil)

// Embed writes the content stream to the PDF file.
// No resource validation is performed; validation is done at the Page level.
func (pc *PageContent) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
	ref := rm.Alloc()
	stm, err := rm.Out().OpenStream(ref, nil, pdf.FilterCompress{})
	if err != nil {
		return nil, err
	}

	for _, op := range pc.Operators {
		if err := content.WriteOperator(stm, op); err != nil {
			stm.Close()
			return nil, err
		}
	}

	if err := stm.Close(); err != nil {
		return nil, err
	}

	return ref, nil
}

// Page represents a PDF page object.
// It implements [pdf.Encoder] for file-dependent embedding.
type Page struct {
	// Required fields
	Parent    pdf.Reference       // parent page tree node
	MediaBox  *pdf.Rectangle      // inheritable
	Resources *content.Resources  // inheritable

	// Content streams
	Contents []*PageContent

	// Optional boxes (PDF 1.3+)
	CropBox  *pdf.Rectangle // inheritable
	BleedBox *pdf.Rectangle
	TrimBox  *pdf.Rectangle
	ArtBox   *pdf.Rectangle

	// Box display (PDF 1.4)
	BoxColorInfo pdf.Object // TODO: implement proper type

	// Rotation
	Rotate int // inheritable, multiple of 90

	// Transparency (PDF 1.4)
	Group pdf.Object // TODO: implement transparency group type

	// Thumbnail
	Thumb *thumbnail.Thumbnail

	// Article beads (PDF 1.1)
	B []pdf.Reference

	// Presentation (PDF 1.1)
	Dur   float64    // display duration in seconds
	Trans pdf.Object // TODO: transition dictionary

	// Annotations
	Annots []annotation.Annotation

	// Actions (PDF 1.2)
	AA pdf.Object // TODO: additional-actions dictionary

	// Metadata (PDF 1.4)
	Metadata *metadata.Stream

	// Page-piece (PDF 1.3)
	LastModified time.Time
	PieceInfo    *pieceinfo.PieceInfo

	// Structure (PDF 1.3)
	StructParents optional.Int

	// Web Capture (PDF 1.3, deprecated in 2.0)
	ID pdf.String
	PZ float64 // preferred zoom

	// Separation (PDF 1.3)
	SeparationInfo pdf.Object // TODO: separation dictionary

	// Tab order (PDF 1.5)
	Tabs pdf.Name // R, C, S, A (2.0), W (2.0)

	// Template (PDF 1.5)
	TemplateInstantiated pdf.Name

	// Navigation (PDF 1.5)
	PresSteps pdf.Object // TODO: navigation node dictionary

	// User units (PDF 1.6)
	UserUnit float64 // default 1.0 = 1/72 inch

	// Viewports (PDF 1.6)
	VP pdf.Object // TODO: array of viewport dictionaries

	// Associated files (PDF 2.0)
	AF pdf.Object // TODO: array of file specification dictionaries

	// Output intents (PDF 2.0)
	OutputIntents pdf.Object // TODO: array of output intent dictionaries

	// Document parts (PDF 2.0)
	DPart pdf.Reference
}

var _ pdf.Encoder = (*Page)(nil)

// Encode writes the page dictionary to the PDF file.
func (p *Page) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	w := rm.Out
	v := pdf.GetVersion(w)

	// Validate all content streams as concatenation
	if len(p.Contents) > 0 {
		cw := content.NewWriter(v, content.Page, p.Resources)
		for _, pc := range p.Contents {
			if err := cw.Validate(pc.Operators); err != nil {
				return nil, err
			}
		}
		if err := cw.Close(); err != nil {
			return nil, err
		}
	}

	// Build the page dictionary
	dict := pdf.Dict{
		"Type":   pdf.Name("Page"),
		"Parent": p.Parent,
	}

	// Required inheritable fields
	if p.MediaBox != nil {
		dict["MediaBox"] = p.MediaBox
	}
	if p.Resources != nil {
		resObj, err := rm.Embed(p.Resources)
		if err != nil {
			return nil, err
		}
		dict["Resources"] = resObj
	}

	// Contents
	if len(p.Contents) == 1 {
		ref, err := rm.Embed(p.Contents[0])
		if err != nil {
			return nil, err
		}
		dict["Contents"] = ref
	} else if len(p.Contents) > 1 {
		arr := make(pdf.Array, len(p.Contents))
		for i, pc := range p.Contents {
			ref, err := rm.Embed(pc)
			if err != nil {
				return nil, err
			}
			arr[i] = ref
		}
		dict["Contents"] = arr
	}

	// Optional boxes
	if p.CropBox != nil {
		dict["CropBox"] = p.CropBox
	}
	if p.BleedBox != nil {
		if err := pdf.CheckVersion(w, "BleedBox", pdf.V1_3); err != nil {
			return nil, err
		}
		dict["BleedBox"] = p.BleedBox
	}
	if p.TrimBox != nil {
		if err := pdf.CheckVersion(w, "TrimBox", pdf.V1_3); err != nil {
			return nil, err
		}
		dict["TrimBox"] = p.TrimBox
	}
	if p.ArtBox != nil {
		if err := pdf.CheckVersion(w, "ArtBox", pdf.V1_3); err != nil {
			return nil, err
		}
		dict["ArtBox"] = p.ArtBox
	}

	// BoxColorInfo
	if p.BoxColorInfo != nil {
		if err := pdf.CheckVersion(w, "BoxColorInfo", pdf.V1_4); err != nil {
			return nil, err
		}
		dict["BoxColorInfo"] = p.BoxColorInfo
	}

	// Rotate
	if p.Rotate != 0 {
		dict["Rotate"] = pdf.Integer(p.Rotate)
	}

	// Group
	if p.Group != nil {
		if err := pdf.CheckVersion(w, "Group", pdf.V1_4); err != nil {
			return nil, err
		}
		dict["Group"] = p.Group
	}

	// Thumb
	if p.Thumb != nil {
		thumbRef, err := rm.Embed(p.Thumb)
		if err != nil {
			return nil, err
		}
		dict["Thumb"] = thumbRef
	}

	// B (article beads)
	if len(p.B) > 0 {
		if err := pdf.CheckVersion(w, "B", pdf.V1_1); err != nil {
			return nil, err
		}
		arr := make(pdf.Array, len(p.B))
		for i, ref := range p.B {
			arr[i] = ref
		}
		dict["B"] = arr
	}

	// Dur
	if p.Dur > 0 {
		if err := pdf.CheckVersion(w, "Dur", pdf.V1_1); err != nil {
			return nil, err
		}
		dict["Dur"] = pdf.Number(p.Dur)
	}

	// Trans
	if p.Trans != nil {
		if err := pdf.CheckVersion(w, "Trans", pdf.V1_1); err != nil {
			return nil, err
		}
		dict["Trans"] = p.Trans
	}

	// Annots
	if len(p.Annots) > 0 {
		arr := make(pdf.Array, len(p.Annots))
		for i, annot := range p.Annots {
			ref, err := annot.Encode(rm)
			if err != nil {
				return nil, err
			}
			arr[i] = ref
		}
		dict["Annots"] = arr
	}

	// AA
	if p.AA != nil {
		if err := pdf.CheckVersion(w, "AA", pdf.V1_2); err != nil {
			return nil, err
		}
		dict["AA"] = p.AA
	}

	// Metadata
	if p.Metadata != nil {
		if err := pdf.CheckVersion(w, "Metadata", pdf.V1_4); err != nil {
			return nil, err
		}
		metaRef, err := rm.Embed(p.Metadata)
		if err != nil {
			return nil, err
		}
		dict["Metadata"] = metaRef
	}

	// LastModified
	if !p.LastModified.IsZero() {
		if err := pdf.CheckVersion(w, "LastModified", pdf.V1_3); err != nil {
			return nil, err
		}
		dict["LastModified"] = pdf.Date(p.LastModified)
	}

	// PieceInfo
	if p.PieceInfo != nil {
		if err := pdf.CheckVersion(w, "PieceInfo", pdf.V1_3); err != nil {
			return nil, err
		}
		pieceRef, err := rm.Embed(p.PieceInfo)
		if err != nil {
			return nil, err
		}
		dict["PieceInfo"] = pieceRef
	}

	// StructParents
	if key, ok := p.StructParents.Get(); ok {
		if err := pdf.CheckVersion(w, "StructParents", pdf.V1_3); err != nil {
			return nil, err
		}
		dict["StructParents"] = key
	}

	// ID (deprecated in 2.0)
	if len(p.ID) > 0 && v < pdf.V2_0 {
		if err := pdf.CheckVersion(w, "ID", pdf.V1_3); err != nil {
			return nil, err
		}
		dict["ID"] = p.ID
	}

	// PZ
	if p.PZ != 0 {
		if err := pdf.CheckVersion(w, "PZ", pdf.V1_3); err != nil {
			return nil, err
		}
		dict["PZ"] = pdf.Number(p.PZ)
	}

	// SeparationInfo
	if p.SeparationInfo != nil {
		if err := pdf.CheckVersion(w, "SeparationInfo", pdf.V1_3); err != nil {
			return nil, err
		}
		dict["SeparationInfo"] = p.SeparationInfo
	}

	// Tabs
	if p.Tabs != "" {
		if err := pdf.CheckVersion(w, "Tabs", pdf.V1_5); err != nil {
			return nil, err
		}
		dict["Tabs"] = p.Tabs
	}

	// TemplateInstantiated
	if p.TemplateInstantiated != "" {
		if err := pdf.CheckVersion(w, "TemplateInstantiated", pdf.V1_5); err != nil {
			return nil, err
		}
		dict["TemplateInstantiated"] = p.TemplateInstantiated
	}

	// PresSteps
	if p.PresSteps != nil {
		if err := pdf.CheckVersion(w, "PresSteps", pdf.V1_5); err != nil {
			return nil, err
		}
		dict["PresSteps"] = p.PresSteps
	}

	// UserUnit
	if p.UserUnit != 0 && p.UserUnit != 1.0 {
		if err := pdf.CheckVersion(w, "UserUnit", pdf.V1_6); err != nil {
			return nil, err
		}
		dict["UserUnit"] = pdf.Number(p.UserUnit)
	}

	// VP
	if p.VP != nil {
		if err := pdf.CheckVersion(w, "VP", pdf.V1_6); err != nil {
			return nil, err
		}
		dict["VP"] = p.VP
	}

	// AF
	if p.AF != nil {
		if err := pdf.CheckVersion(w, "AF", pdf.V2_0); err != nil {
			return nil, err
		}
		dict["AF"] = p.AF
	}

	// OutputIntents
	if p.OutputIntents != nil {
		if err := pdf.CheckVersion(w, "OutputIntents", pdf.V2_0); err != nil {
			return nil, err
		}
		dict["OutputIntents"] = p.OutputIntents
	}

	// DPart
	if p.DPart != 0 {
		if err := pdf.CheckVersion(w, "DPart", pdf.V2_0); err != nil {
			return nil, err
		}
		dict["DPart"] = p.DPart
	}

	return dict, nil
}

// ExtractContent reads a single content stream from a PDF object.
func ExtractContent(x *pdf.Extractor, obj pdf.Object) (*PageContent, error) {
	resolved, err := x.Resolve(obj)
	if err != nil {
		return nil, err
	}

	stream, ok := resolved.(*pdf.Stream)
	if !ok {
		return nil, pdf.Errorf("expected stream, got %T", resolved)
	}

	stm, err := pdf.DecodeStream(x.R, stream, 0)
	if err != nil {
		return nil, err
	}
	defer stm.Close()

	operators, err := content.ReadStream(stm, pdf.GetVersion(x.R), content.Page)
	if err != nil {
		return nil, err
	}

	return &PageContent{Operators: operators}, nil
}

// Decode reads a page dictionary from a PDF object.
// The page dictionary should already have inherited attributes resolved
// (e.g., via [pagetree.Iterator]).
func Decode(x *pdf.Extractor, obj pdf.Object) (*Page, error) {
	dict, err := x.GetDictTyped(obj, "Page")
	if err != nil {
		return nil, err
	}
	if dict == nil {
		return nil, pdf.Error("missing page dictionary")
	}

	p := &Page{}

	// Parent (required)
	if parent, ok := dict["Parent"].(pdf.Reference); ok {
		p.Parent = parent
	}

	// MediaBox (required, inheritable)
	if mediaBox, err := pdf.GetRectangle(x.R, dict["MediaBox"]); err != nil {
		return nil, err
	} else if mediaBox != nil {
		p.MediaBox = mediaBox
	}

	// Resources (required, inheritable)
	if res, err := extract.Resources(x, dict["Resources"]); err != nil {
		return nil, err
	} else {
		p.Resources = res
	}

	// Contents (optional)
	if contentsObj := dict["Contents"]; contentsObj != nil {
		resolved, err := x.Resolve(contentsObj)
		if err != nil {
			return nil, err
		}
		switch c := resolved.(type) {
		case *pdf.Stream:
			// Single stream
			pc, err := ExtractContent(x, contentsObj)
			if err != nil {
				return nil, err
			}
			p.Contents = []*PageContent{pc}
		case pdf.Array:
			// Array of streams
			for _, item := range c {
				pc, err := ExtractContent(x, item)
				if err != nil {
					return nil, err
				}
				p.Contents = append(p.Contents, pc)
			}
		}
	}

	// CropBox (optional, inheritable)
	if cropBox, err := pdf.GetRectangle(x.R, dict["CropBox"]); err != nil {
		return nil, err
	} else if cropBox != nil {
		p.CropBox = cropBox
	}

	// BleedBox (optional)
	if bleedBox, err := pdf.GetRectangle(x.R, dict["BleedBox"]); err != nil {
		return nil, err
	} else if bleedBox != nil {
		p.BleedBox = bleedBox
	}

	// TrimBox (optional)
	if trimBox, err := pdf.GetRectangle(x.R, dict["TrimBox"]); err != nil {
		return nil, err
	} else if trimBox != nil {
		p.TrimBox = trimBox
	}

	// ArtBox (optional)
	if artBox, err := pdf.GetRectangle(x.R, dict["ArtBox"]); err != nil {
		return nil, err
	} else if artBox != nil {
		p.ArtBox = artBox
	}

	// BoxColorInfo (optional)
	p.BoxColorInfo = dict["BoxColorInfo"]

	// Rotate (optional, inheritable)
	if rotate, err := pdf.Optional(x.GetInteger(dict["Rotate"])); err != nil {
		return nil, err
	} else {
		p.Rotate = int(rotate)
	}

	// Group (optional)
	p.Group = dict["Group"]

	// Thumb (optional)
	if thumb, err := pdf.ExtractorGetOptional(x, dict["Thumb"], thumbnail.ExtractThumbnail); err != nil {
		return nil, err
	} else {
		p.Thumb = thumb
	}

	// B (optional)
	if bArray, err := pdf.Optional(x.GetArray(dict["B"])); err != nil {
		return nil, err
	} else if bArray != nil {
		for _, item := range bArray {
			if ref, ok := item.(pdf.Reference); ok {
				p.B = append(p.B, ref)
			}
		}
	}

	// Dur (optional)
	if dur, err := pdf.Optional(x.GetNumber(dict["Dur"])); err != nil {
		return nil, err
	} else {
		p.Dur = dur
	}

	// Trans (optional)
	p.Trans = dict["Trans"]

	// Annots (optional)
	if annotsArray, err := pdf.Optional(x.GetArray(dict["Annots"])); err != nil {
		return nil, err
	} else if annotsArray != nil {
		for _, item := range annotsArray {
			annot, err := annotation.Decode(x, item)
			if err != nil {
				// permissive: skip invalid annotations
				continue
			}
			p.Annots = append(p.Annots, annot)
		}
	}

	// AA (optional)
	p.AA = dict["AA"]

	// Metadata (optional)
	if meta, err := pdf.Optional(metadata.Extract(x.R, dict["Metadata"])); err != nil {
		return nil, err
	} else {
		p.Metadata = meta
	}

	// LastModified (optional)
	if lastMod, err := pdf.Optional(pdf.GetDate(x.R, dict["LastModified"])); err != nil {
		return nil, err
	} else if !time.Time(lastMod).IsZero() {
		p.LastModified = time.Time(lastMod)
	}

	// PieceInfo (optional)
	if piece, err := pdf.Optional(pieceinfo.Extract(x.R, dict["PieceInfo"])); err != nil {
		return nil, err
	} else {
		p.PieceInfo = piece
	}

	// StructParents (optional)
	if dict["StructParents"] != nil {
		if key, err := pdf.Optional(x.GetInteger(dict["StructParents"])); err != nil {
			return nil, err
		} else {
			p.StructParents.Set(key)
		}
	}

	// ID (optional, deprecated)
	if id, err := pdf.Optional(pdf.GetString(x.R, dict["ID"])); err != nil {
		return nil, err
	} else {
		p.ID = id
	}

	// PZ (optional)
	if pz, err := pdf.Optional(x.GetNumber(dict["PZ"])); err != nil {
		return nil, err
	} else {
		p.PZ = pz
	}

	// SeparationInfo (optional)
	p.SeparationInfo = dict["SeparationInfo"]

	// Tabs (optional)
	if tabs, err := pdf.Optional(x.GetName(dict["Tabs"])); err != nil {
		return nil, err
	} else {
		p.Tabs = tabs
	}

	// TemplateInstantiated (optional)
	if tmpl, err := pdf.Optional(x.GetName(dict["TemplateInstantiated"])); err != nil {
		return nil, err
	} else {
		p.TemplateInstantiated = tmpl
	}

	// PresSteps (optional)
	p.PresSteps = dict["PresSteps"]

	// UserUnit (optional)
	if userUnit, err := pdf.Optional(x.GetNumber(dict["UserUnit"])); err != nil {
		return nil, err
	} else {
		p.UserUnit = userUnit
	}

	// VP (optional)
	p.VP = dict["VP"]

	// AF (optional)
	p.AF = dict["AF"]

	// OutputIntents (optional)
	p.OutputIntents = dict["OutputIntents"]

	// DPart (optional)
	if dpart, ok := dict["DPart"].(pdf.Reference); ok {
		p.DPart = dpart
	}

	return p, nil
}
