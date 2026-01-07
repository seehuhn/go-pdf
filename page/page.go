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
package page

import (
	"errors"
	"fmt"
	"time"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/action/triggers"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/file"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/extract"
	"seehuhn.de/go/pdf/graphics/group"
	"seehuhn.de/go/pdf/graphics/image/thumbnail"
	"seehuhn.de/go/pdf/metadata"
	"seehuhn.de/go/pdf/optional"
	"seehuhn.de/go/pdf/page/boxcolor"
	"seehuhn.de/go/pdf/page/navnode"
	"seehuhn.de/go/pdf/page/separation"
	"seehuhn.de/go/pdf/page/transition"
	"seehuhn.de/go/pdf/pieceinfo"
)

// PDF 2.0 sections: 7.7.3

// Page represents a PDF page object (Table 31 in the PDF spec).
// It implements [pdf.Encoder] for file-dependent embedding.
//
// This type does not support invisible Template pages (Type=Template).
type Page struct {
	// Parent is the page tree node that is the immediate parent of this page.
	Parent pdf.Reference

	// MediaBox (inheritable) defines the boundaries of the physical medium
	// on which the page is displayed or printed.
	MediaBox *pdf.Rectangle

	// Resources (inheritable) contains resources required by the page contents.
	Resources *content.Resources

	// Contents (optional) holds content streams describing the page's appearance.
	Contents []*Content

	// CropBox (optional; inheritable) defines the visible region of the page.
	// Default: MediaBox.
	CropBox *pdf.Rectangle

	// BleedBox (optional) defines the clipping region for production output.
	// Default: CropBox.
	BleedBox *pdf.Rectangle

	// TrimBox (optional) defines the intended dimensions after trimming.
	// Default: CropBox.
	TrimBox *pdf.Rectangle

	// ArtBox (optional) defines the extent of the page's meaningful content.
	// Default: CropBox.
	ArtBox *pdf.Rectangle

	// BoxColorInfo (optional) specifies colors for displaying page boundary
	// guidelines on screen.
	BoxColorInfo *boxcolor.Info

	// Rotate (optional; inheritable) specifies clockwise rotation in degrees.
	// Must be a multiple of 90.
	Rotate int

	// Group (optional) specifies transparency group attributes for the page.
	Group *group.TransparencyAttributes

	// Thumbnail (optional) is a thumbnail image for the page.
	//
	// This corresponds to the /Thumb entry in the PDF dictionary.
	Thumbnail *thumbnail.Thumbnail

	// ArticleBeads (optional) lists article beads on the page, in reading order.
	//
	// This corresponds to the /B entry in the PDF dictionary.
	ArticleBeads []pdf.Reference

	// Duration (optional) is the display duration in seconds for presentations.
	//
	// This corresponds to the /Dur entry in the PDF dictionary.
	Duration float64

	// Transition (optional) describes the transition effect for presentations.
	//
	// This corresponds to the /Trans entry in the PDF dictionary.
	Transition *transition.Transition

	// Annots (optional) lists annotations associated with the page.
	Annots []annotation.Annotation

	// AA (optional) defines additional actions for page open/close events.
	AA *triggers.Page

	// Metadata (optional) is a metadata stream for the page.
	Metadata *metadata.Stream

	// LastModified (optional) is when the page contents were last modified.
	// Required if PieceInfo is present.
	LastModified time.Time

	// PieceInfo (optional) holds application-specific page data.
	PieceInfo *pieceinfo.PieceInfo

	// StructParents (optional) is this page's key in the structural parent tree.
	// Required if the page contains structural content items.
	StructParents optional.Int

	// ID (optional; deprecated in PDF 2.0) is the digital identifier of the
	// page's parent Web Capture content set.
	ID pdf.String

	// PZ (optional) is the preferred zoom factor for the page.
	PZ float64

	// SeparationInfo (optional) contains color separation information
	// for preseparated PDF files (deprecated with PDF 2.0).
	SeparationInfo *separation.Dict

	// Tabs (optional) specifies the tab order for annotations.
	// Values: R (row), C (column), S (structure), A (array order; PDF 2.0),
	// W (widget order; PDF 2.0).
	Tabs pdf.Name

	// TemplateInstantiated (optional) is the name of the originating named page
	// object, if this page was created from one.
	TemplateInstantiated pdf.Name

	// PresSteps (optional) is the list of navigation nodes for sub-page navigation.
	// Navigation nodes allow navigating between different states of the same page
	// during presentations (PDF 1.5).
	PresSteps []*navnode.Node

	// UserUnit (optional) is the size of user space units in multiples of
	// 1/72 inch.
	//
	// When writing, 0 is treated as 1.0.
	UserUnit float64

	// VP (optional) specifies rectangular viewport regions of the page.
	VP pdf.Object // TODO: array of viewport dictionaries

	// AF (optional; PDF 2.0) lists associated files for this page.
	AF []*file.Specification

	// OutputIntents (optional; PDF 2.0) specifies color characteristics
	// for output devices.
	OutputIntents pdf.Object // TODO: array of output intent dictionaries

	// DPart (optional; PDF 2.0) references the DPart whose range includes
	// this page. Required if this page is within a DPart range.
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
		obj, err := rm.Embed(p.BoxColorInfo)
		if err != nil {
			return nil, err
		}
		dict["BoxColorInfo"] = obj
	}

	// Rotate (must be multiple of 90)
	if p.Rotate != 0 {
		if p.Rotate%90 != 0 {
			return nil, fmt.Errorf("Rotate must be a multiple of 90, got %d", p.Rotate)
		}
		dict["Rotate"] = pdf.Integer(p.Rotate)
	}

	// Group
	if p.Group != nil {
		groupObj, err := rm.Embed(p.Group)
		if err != nil {
			return nil, err
		}
		dict["Group"] = groupObj
	}

	// Thumb
	if p.Thumbnail != nil {
		thumbRef, err := rm.Embed(p.Thumbnail)
		if err != nil {
			return nil, err
		}
		dict["Thumb"] = thumbRef
	}

	// B (article beads)
	if len(p.ArticleBeads) > 0 {
		if err := pdf.CheckVersion(w, "B", pdf.V1_1); err != nil {
			return nil, err
		}
		arr := make(pdf.Array, len(p.ArticleBeads))
		for i, ref := range p.ArticleBeads {
			arr[i] = ref
		}
		dict["B"] = arr
	}

	// Dur
	if p.Duration > 0 {
		if err := pdf.CheckVersion(w, "Dur", pdf.V1_1); err != nil {
			return nil, err
		}
		dict["Dur"] = pdf.Number(p.Duration)
	}

	// Trans
	if p.Transition != nil {
		transObj, err := rm.Embed(p.Transition)
		if err != nil {
			return nil, err
		}
		dict["Trans"] = transObj
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
		obj, err := p.AA.Encode(rm)
		if err != nil {
			return nil, err
		}
		if obj != nil {
			dict["AA"] = obj
		}
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

	// PieceInfo (requires LastModified)
	if p.PieceInfo != nil {
		if err := pdf.CheckVersion(w, "PieceInfo", pdf.V1_3); err != nil {
			return nil, err
		}
		if p.LastModified.IsZero() {
			return nil, errors.New("LastModified is required when PieceInfo is present")
		}
		pieceRef, err := rm.Embed(p.PieceInfo)
		if err != nil {
			return nil, err
		}
		dict["PieceInfo"] = pieceRef
	}

	// LastModified
	if !p.LastModified.IsZero() {
		if err := pdf.CheckVersion(w, "LastModified", pdf.V1_3); err != nil {
			return nil, err
		}
		dict["LastModified"] = pdf.Date(p.LastModified)
	}

	// StructParents
	if key, ok := p.StructParents.Get(); ok {
		if err := pdf.CheckVersion(w, "StructParents", pdf.V1_3); err != nil {
			return nil, err
		}
		dict["StructParents"] = key
	}

	// ID (deprecated in 2.0)
	if len(p.ID) > 0 {
		if v >= pdf.V2_0 {
			return nil, errors.New("ID is deprecated in PDF 2.0")
		}
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
		sepObj, err := p.SeparationInfo.Encode(rm)
		if err != nil {
			return nil, err
		}
		dict["SeparationInfo"] = sepObj
	}

	// Tabs (R, C, S for PDF 1.5+; A, W for PDF 2.0+)
	if p.Tabs != "" {
		if err := pdf.CheckVersion(w, "Tabs", pdf.V1_5); err != nil {
			return nil, err
		}
		switch p.Tabs {
		case "R", "C", "S":
			// valid for PDF 1.5+
		case "A", "W":
			if err := pdf.CheckVersion(w, "Tabs value "+string(p.Tabs), pdf.V2_0); err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("invalid Tabs value %q", p.Tabs)
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
	if len(p.PresSteps) > 0 {
		presSteps, err := navnode.Encode(rm, p.PresSteps)
		if err != nil {
			return nil, pdf.Wrap(err, "PresSteps")
		}
		dict["PresSteps"] = presSteps
	}

	// UserUnit (0 is shorthand for 1.0; must be positive)
	if p.UserUnit < 0 {
		return nil, fmt.Errorf("UserUnit must be positive, got %g", p.UserUnit)
	}
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
	if len(p.AF) > 0 {
		if err := pdf.CheckVersion(w, "page AF entry", pdf.V2_0); err != nil {
			return nil, err
		}

		// validate each file specification can be used as associated file
		for i, spec := range p.AF {
			if spec == nil {
				continue
			}
			if err := spec.CanBeAF(v); err != nil {
				return nil, fmt.Errorf("AF[%d]: %w", i, err)
			}
		}

		// embed the file specifications
		var afArray pdf.Array
		for _, spec := range p.AF {
			if spec != nil {
				embedded, err := rm.Embed(spec)
				if err != nil {
					return nil, err
				}
				afArray = append(afArray, embedded)
			}
		}
		dict["AF"] = afArray
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
			p.Contents = []*Content{pc}
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
	if bci, err := pdf.ExtractorGetOptional(x, dict["BoxColorInfo"], boxcolor.ExtractInfo); err != nil {
		return nil, err
	} else {
		p.BoxColorInfo = bci
	}

	// Rotate (optional, inheritable)
	if rotate, err := pdf.Optional(x.GetInteger(dict["Rotate"])); err != nil {
		return nil, err
	} else {
		p.Rotate = int(rotate)
	}

	// Group (optional)
	if dict["Group"] != nil {
		grp, err := group.ExtractTransparencyAttributes(x, dict["Group"])
		if err != nil {
			return nil, err
		}
		p.Group = grp
	}

	// Thumb (optional)
	if thumb, err := pdf.ExtractorGetOptional(x, dict["Thumb"], thumbnail.ExtractThumbnail); err != nil {
		return nil, err
	} else {
		p.Thumbnail = thumb
	}

	// B (optional)
	if bArray, err := pdf.Optional(x.GetArray(dict["B"])); err != nil {
		return nil, err
	} else if bArray != nil {
		for _, item := range bArray {
			if ref, ok := item.(pdf.Reference); ok {
				p.ArticleBeads = append(p.ArticleBeads, ref)
			}
		}
	}

	// Dur (optional)
	if dur, err := pdf.Optional(x.GetNumber(dict["Dur"])); err != nil {
		return nil, err
	} else {
		p.Duration = dur
	}

	// Trans (optional)
	if dict["Trans"] != nil {
		trans, err := transition.Extract(x, dict["Trans"])
		if err != nil {
			return nil, err
		}
		p.Transition = trans
	}

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
	if aa, err := triggers.DecodePage(x, dict["AA"]); err != nil {
		return nil, err
	} else {
		p.AA = aa
	}

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
	if dict["SeparationInfo"] != nil {
		sep, err := separation.Decode(x, dict["SeparationInfo"])
		if err != nil {
			return nil, err
		}
		p.SeparationInfo = sep
	}

	// Tabs (optional; only accept valid values)
	if tabs, err := pdf.Optional(x.GetName(dict["Tabs"])); err != nil {
		return nil, err
	} else {
		switch tabs {
		case "R", "C", "S", "A", "W":
			p.Tabs = tabs
		default:
			// invalid or empty value - leave as empty
		}
	}

	// TemplateInstantiated (optional)
	if tmpl, err := pdf.Optional(x.GetName(dict["TemplateInstantiated"])); err != nil {
		return nil, err
	} else {
		p.TemplateInstantiated = tmpl
	}

	// PresSteps (optional)
	if presSteps, err := navnode.Decode(x, dict["PresSteps"]); err != nil {
		return nil, pdf.Wrap(err, "PresSteps")
	} else {
		p.PresSteps = presSteps
	}

	// UserUnit (optional; default 1.0)
	if userUnit, err := pdf.Optional(x.GetNumber(dict["UserUnit"])); err != nil {
		return nil, err
	} else if userUnit > 0 {
		p.UserUnit = userUnit
	} else {
		p.UserUnit = 1.0
	}

	// VP (optional)
	p.VP = dict["VP"]

	// AF (optional)
	if afArray, err := pdf.Optional(x.GetArray(dict["AF"])); err != nil {
		return nil, err
	} else if afArray != nil {
		p.AF = make([]*file.Specification, 0, len(afArray))
		for _, afObj := range afArray {
			if spec, err := pdf.ExtractorGetOptional(x, afObj, file.ExtractSpecification); err != nil {
				return nil, err
			} else if spec != nil {
				p.AF = append(p.AF, spec)
			}
		}
	}

	// OutputIntents (optional)
	p.OutputIntents = dict["OutputIntents"]

	// DPart (optional)
	if dpart, ok := dict["DPart"].(pdf.Reference); ok {
		p.DPart = dpart
	}

	return p, nil
}
