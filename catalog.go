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

package pdf

import (
	"errors"
	"fmt"

	"golang.org/x/text/language"
)

// Catalog represents a PDF Document Catalog.  The only required field in this
// structure is Pages, which specifies the root of the page tree.
//
// Use [DecodeCatalog] (typically via [Decode]) to read a catalog from a
// PDF file and [Catalog.Encode] to produce its dictionary form for writing.
//
// The Document Catalog is documented in section 7.7.2 of PDF 32000-1:2008.
type Catalog struct {
	// Version (optional, PDF 1.4) specifies the PDF version this document
	// conforms to if later than the version in the file header.
	Version Version

	// Extensions (optional, PDF 1.7) contains developer extensions information
	// for extensions that occur in this document.
	Extensions Object

	// Pages is the root of the document's page tree.
	Pages Reference

	// PageLabels (optional, PDF 1.3) defines the page labeling for the
	// document as a number tree where keys are page indices and values are
	// page label dictionaries.
	PageLabels Object

	// Names (optional, PDF 1.2) is the document's name dictionary.
	Names Object

	// Dests (optional, PDF 1.1) contains a dictionary of names and
	// corresponding destinations.
	Dests Object

	// ViewerPreferences (optional, PDF 1.2) specifies how the document should
	// be displayed on screen.
	ViewerPreferences Object

	// PageLayout (optional) specifies the page layout to use when the document
	// is opened. Valid values are SinglePage, OneColumn, TwoColumnLeft,
	// TwoColumnRight, TwoPageLeft, TwoPageRight.
	PageLayout Name

	// PageMode (optional) specifies how the document should be displayed when
	// opened. Valid values are UseNone, UseOutlines, UseThumbs, FullScreen,
	// UseOC, UseAttachments.
	PageMode Name

	// Outlines (optional) is the root of the document's outline hierarchy.
	Outlines Reference

	// Threads (optional, PDF 1.1) contains an array of thread dictionaries
	// representing the document's article threads.
	Threads Reference

	// OpenAction (optional, PDF 1.1) specifies a destination to display or
	// action to perform when the document is opened.
	OpenAction Object

	// AA (optional, PDF 1.2) defines additional actions to take in response to
	// various trigger events affecting the document.
	AA Object

	// URI (optional, PDF 1.1) contains document-level information for URI
	// actions.
	URI Object

	// AcroForm (optional, PDF 1.2) is the document's interactive form
	// dictionary.
	AcroForm Object

	// Metadata (optional, PDF 1.4) contains the document-level XMP metadata
	// stream.  Populated by [DecodeCatalog] (eager decode); embedded by
	// [Catalog.Encode] when writing.
	Metadata *MetadataStream

	// StructTreeRoot (optional, PDF 1.3) is the document's structure tree root
	// dictionary.
	StructTreeRoot Object

	// MarkInfo (optional, PDF 1.4) contains information about the document's
	// usage of tagged PDF conventions.
	MarkInfo Object

	// Lang (optional, PDF 1.4) specifies the natural language for all text in
	// the document.
	Lang language.Tag

	// SpiderInfo (optional, PDF 1.3) contains Web Capture information and state.
	SpiderInfo Object

	// OutputIntents (optional, PDF 1.4) specifies the color characteristics of
	// output devices on which the document might be rendered.
	OutputIntents Object

	// PieceInfo (optional, PDF 1.4) is a page-piece dictionary associated with
	// the document.
	PieceInfo Object

	// OCProperties (optional, PDF 1.5) contains the document's optional
	// content properties. Required if the document contains optional content.
	OCProperties Object

	// Perms (optional, PDF 1.5) specifies user access permissions for the
	// document.
	Perms Object

	// Legal (optional, PDF 1.5) contains attestations regarding the content of
	// the PDF document as it relates to the legality of digital signatures.
	Legal Object

	// Requirements (optional, PDF 1.7) contains an array of requirement
	// dictionaries that represent requirements for the document.
	Requirements Object

	// Collection (optional, PDF 1.7) enhances the presentation of file
	// attachments stored in the PDF document.
	Collection Object

	// NeedsRendering (optional, PDF 1.5, deprecated in PDF 2.0) specifies
	// whether the document should be regenerated when first opened. Used
	// for XFA forms.
	NeedsRendering bool

	// DSS (optional, PDF 2.0) contains document-wide security information.
	DSS Object

	// AF (optional, PDF 2.0) contains an array of file specification
	// dictionaries denoting the associated files for this PDF document.
	AF Object

	// DPartRoot (optional, PDF 2.0) describes the document parts hierarchy for
	// this PDF document.
	DPartRoot Object
}

// DecodeCatalog reads the document catalog dictionary and returns its
// Go representation.
//
// The Metadata stream, if present, is decoded eagerly so that the
// returned [Catalog.Metadata] field is populated with a typed
// [*MetadataStream].
func DecodeCatalog(c Cursor, obj Object, _ bool) (*Catalog, error) {
	dict, err := c.DictTyped(obj, "Catalog")
	if err != nil {
		return nil, err
	} else if dict == nil {
		return nil, &MalformedFileError{
			Err: errors.New("missing catalog dictionary"),
		}
	}

	// required Pages field
	pagesObj := dict["Pages"]
	if pagesObj == nil {
		return nil, &MalformedFileError{
			Err: errors.New("required field Pages is missing"),
		}
	}

	// permissively accept missing/malformed Pages reference
	var pages Reference
	if ref, ok := pagesObj.(Reference); ok {
		pages = ref
	}

	// Version (optional, PDF 1.4)
	var version Version
	if dict["Version"] != nil {
		var vString string
		switch v := dict["Version"].(type) {
		case Name:
			vString = string(v)
		case String:
			vString = string(v)
		case Real:
			vString = fmt.Sprintf("%.1f", v)
		}
		if vString != "" {
			if v, err := ParseVersion(vString); err == nil {
				version = v
			}
		}
	}

	pageLayout, _ := c.Name(dict["PageLayout"])
	pageMode, _ := c.Name(dict["PageMode"])

	var outlines Reference
	if ref, ok := dict["Outlines"].(Reference); ok {
		outlines = ref
	}

	var threads Reference
	if ref, ok := dict["Threads"].(Reference); ok {
		threads = ref
	}

	var lang language.Tag
	if dict["Lang"] != nil {
		langStr, err := c.TextString(dict["Lang"])
		if err == nil && langStr != "" {
			lang, _ = language.Parse(string(langStr))
		}
	}

	needsRendering, _ := c.Boolean(dict["NeedsRendering"])

	// Metadata stream (eager decode; PDF 1.4)
	metadata, err := DecodeOptional(c, dict["Metadata"], ExtractMetadataStream)
	if err != nil {
		return nil, err
	}

	return &Catalog{
		Version:           version,
		Extensions:        dict["Extensions"],
		Pages:             pages,
		PageLabels:        dict["PageLabels"],
		Names:             dict["Names"],
		Dests:             dict["Dests"],
		ViewerPreferences: dict["ViewerPreferences"],
		PageLayout:        pageLayout,
		PageMode:          pageMode,
		Outlines:          outlines,
		Threads:           threads,
		OpenAction:        dict["OpenAction"],
		AA:                dict["AA"],
		URI:               dict["URI"],
		AcroForm:          dict["AcroForm"],
		Metadata:          metadata,
		StructTreeRoot:    dict["StructTreeRoot"],
		MarkInfo:          dict["MarkInfo"],
		Lang:              lang,
		SpiderInfo:        dict["SpiderInfo"],
		OutputIntents:     dict["OutputIntents"],
		PieceInfo:         dict["PieceInfo"],
		OCProperties:      dict["OCProperties"],
		Perms:             dict["Perms"],
		Legal:             dict["Legal"],
		Requirements:      dict["Requirements"],
		Collection:        dict["Collection"],
		NeedsRendering:    bool(needsRendering),
		DSS:               dict["DSS"],
		AF:                dict["AF"],
		DPartRoot:         dict["DPartRoot"],
	}, nil
}

// Encode converts the document catalog to its PDF dictionary representation.
//
// If [Catalog.Metadata] is non-nil, the stream is embedded in the file using
// rm; identical streams are deduplicated by the resource manager.
func (c *Catalog) Encode(rm *ResourceManager) (Native, error) {
	out := rm.Out

	if c.Pages == 0 {
		return nil, errors.New("Catalog: missing Pages reference")
	}

	dict := Dict{
		"Type":  Name("Catalog"),
		"Pages": c.Pages,
	}

	if c.Version != 0 {
		if err := CheckVersion(out, "Catalog Version entry", V1_4); err != nil {
			return nil, err
		}
		if vs, err := c.Version.ToString(); err == nil {
			dict["Version"] = Name(vs)
		}
	}

	if c.Extensions != nil {
		if err := CheckVersion(out, "Catalog Extensions entry", V1_7); err != nil {
			return nil, err
		}
		dict["Extensions"] = c.Extensions
	}

	if c.PageLabels != nil {
		if err := CheckVersion(out, "Catalog PageLabels entry", V1_3); err != nil {
			return nil, err
		}
		dict["PageLabels"] = c.PageLabels
	}

	if c.Names != nil {
		if err := CheckVersion(out, "Catalog Names entry", V1_2); err != nil {
			return nil, err
		}
		dict["Names"] = c.Names
	}

	if c.Dests != nil {
		if err := CheckVersion(out, "Catalog Dests entry", V1_1); err != nil {
			return nil, err
		}
		dict["Dests"] = c.Dests
	}

	if c.ViewerPreferences != nil {
		if err := CheckVersion(out, "Catalog ViewerPreferences entry", V1_2); err != nil {
			return nil, err
		}
		dict["ViewerPreferences"] = c.ViewerPreferences
	}

	if c.PageLayout != "" {
		dict["PageLayout"] = c.PageLayout
	}

	if c.PageMode != "" {
		dict["PageMode"] = c.PageMode
	}

	if c.Outlines != 0 {
		dict["Outlines"] = c.Outlines
	}

	if c.Threads != 0 {
		if err := CheckVersion(out, "Catalog Threads entry", V1_1); err != nil {
			return nil, err
		}
		dict["Threads"] = c.Threads
	}

	if c.OpenAction != nil {
		if err := CheckVersion(out, "Catalog OpenAction entry", V1_1); err != nil {
			return nil, err
		}
		dict["OpenAction"] = c.OpenAction
	}

	if c.AA != nil {
		if err := CheckVersion(out, "Catalog AA entry", V1_2); err != nil {
			return nil, err
		}
		dict["AA"] = c.AA
	}

	if c.URI != nil {
		if err := CheckVersion(out, "Catalog URI entry", V1_1); err != nil {
			return nil, err
		}
		dict["URI"] = c.URI
	}

	if c.AcroForm != nil {
		if err := CheckVersion(out, "Catalog AcroForm entry", V1_2); err != nil {
			return nil, err
		}
		dict["AcroForm"] = c.AcroForm
	}

	if c.Metadata != nil {
		if err := CheckVersion(out, "Catalog Metadata entry", V1_4); err != nil {
			return nil, err
		}
		ref, err := rm.Embed(c.Metadata)
		if err != nil {
			return nil, err
		}
		dict["Metadata"] = ref
	}

	if c.StructTreeRoot != nil {
		if err := CheckVersion(out, "Catalog StructTreeRoot entry", V1_3); err != nil {
			return nil, err
		}
		dict["StructTreeRoot"] = c.StructTreeRoot
	}

	if c.MarkInfo != nil {
		if err := CheckVersion(out, "Catalog MarkInfo entry", V1_4); err != nil {
			return nil, err
		}
		dict["MarkInfo"] = c.MarkInfo
	}

	if !c.Lang.IsRoot() {
		if err := CheckVersion(out, "Catalog Lang entry", V1_4); err != nil {
			return nil, err
		}
		dict["Lang"] = TextString(c.Lang.String())
	}

	if c.SpiderInfo != nil {
		if err := CheckVersion(out, "Catalog SpiderInfo entry", V1_3); err != nil {
			return nil, err
		}
		dict["SpiderInfo"] = c.SpiderInfo
	}

	if c.OutputIntents != nil {
		if err := CheckVersion(out, "Catalog OutputIntents entry", V1_4); err != nil {
			return nil, err
		}
		dict["OutputIntents"] = c.OutputIntents
	}

	if c.PieceInfo != nil {
		if err := CheckVersion(out, "Catalog PieceInfo entry", V1_4); err != nil {
			return nil, err
		}
		dict["PieceInfo"] = c.PieceInfo
	}

	if c.OCProperties != nil {
		if err := CheckVersion(out, "Catalog OCProperties entry", V1_5); err != nil {
			return nil, err
		}
		dict["OCProperties"] = c.OCProperties
	}

	if c.Perms != nil {
		if err := CheckVersion(out, "Catalog Perms entry", V1_5); err != nil {
			return nil, err
		}
		dict["Perms"] = c.Perms
	}

	if c.Legal != nil {
		if err := CheckVersion(out, "Catalog Legal entry", V1_5); err != nil {
			return nil, err
		}
		dict["Legal"] = c.Legal
	}

	if c.Requirements != nil {
		if err := CheckVersion(out, "Catalog Requirements entry", V1_7); err != nil {
			return nil, err
		}
		dict["Requirements"] = c.Requirements
	}

	if c.Collection != nil {
		if err := CheckVersion(out, "Catalog Collection entry", V1_7); err != nil {
			return nil, err
		}
		dict["Collection"] = c.Collection
	}

	if c.NeedsRendering {
		if err := CheckVersion(out, "Catalog NeedsRendering entry", V1_5); err != nil {
			return nil, err
		}
		dict["NeedsRendering"] = Boolean(true)
	}

	if c.DSS != nil {
		if err := CheckVersion(out, "Catalog DSS entry", V2_0); err != nil {
			return nil, err
		}
		dict["DSS"] = c.DSS
	}

	if c.AF != nil {
		if err := CheckVersion(out, "Catalog AF entry", V2_0); err != nil {
			return nil, err
		}
		dict["AF"] = c.AF
	}

	if c.DPartRoot != nil {
		if err := CheckVersion(out, "Catalog DPartRoot entry", V2_0); err != nil {
			return nil, err
		}
		dict["DPartRoot"] = c.DPartRoot
	}

	return dict, nil
}
