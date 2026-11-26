// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package pdf

import (
	"errors"
	"fmt"

	"golang.org/x/text/language"
	"seehuhn.de/go/pdf/internal/langtag"
)

// Catalog represents a PDF Document Catalog.  The only required field in this
// structure is Pages, which specifies the root of the page tree.
//
// Use [DecodeCatalog] (typically via [Cursor.Decode]) to read a catalog from a
// PDF file and [Catalog.Encode] to produce its dictionary form for writing.
//
// The Document Catalog is documented in section 7.7.2 of PDF 32000-1:2008.
type Catalog struct {
	// Version (optional, PDF 1.4) specifies the PDF version this document
	// conforms to if later than the version in the file header.
	//
	// In an incremental update a zero Version leaves the document version
	// unchanged, and Close rejects a Version below the document version.
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
	// The root tag [language.Und] means no language is given.
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
	// The /Type entry is not checked: the trailer's /Root entry already
	// identifies the dictionary as the catalog, and the only valid value
	// is written back on encode.
	dict, err := c.Dict(obj)
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
		if err == nil {
			lang = langtag.Parse(langStr.String())
		}
	}

	needsRendering, _, err := c.OptionalBoolean(dict["NeedsRendering"])
	if err != nil {
		return nil, err
	}

	// Metadata stream (eager decode; PDF 1.4)
	metadata, err := c.DecodeOptional(dict["Metadata"], ExtractMetadataStream)
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

// catalogEntryVersions lists the catalog entries which are restricted to a
// minimum PDF version, together with a test whether the entry is present.
var catalogEntryVersions = []struct {
	name    string
	version Version
	present func(c *Catalog) bool
}{
	{"PageLayout TwoPageLeft", V1_5, func(c *Catalog) bool { return c.PageLayout == "TwoPageLeft" }},
	{"PageLayout TwoPageRight", V1_5, func(c *Catalog) bool { return c.PageLayout == "TwoPageRight" }},
	{"PageMode UseOC", V1_5, func(c *Catalog) bool { return c.PageMode == "UseOC" }},
	{"PageMode UseAttachments", V1_6, func(c *Catalog) bool { return c.PageMode == "UseAttachments" }},
	{"Version", V1_4, func(c *Catalog) bool { return c.Version != 0 }},
	{"Extensions", V1_7, func(c *Catalog) bool { return c.Extensions != nil }},
	{"PageLabels", V1_3, func(c *Catalog) bool { return c.PageLabels != nil }},
	{"Names", V1_2, func(c *Catalog) bool { return c.Names != nil }},
	{"Dests", V1_1, func(c *Catalog) bool { return c.Dests != nil }},
	{"ViewerPreferences", V1_2, func(c *Catalog) bool { return c.ViewerPreferences != nil }},
	{"Threads", V1_1, func(c *Catalog) bool { return c.Threads != 0 }},
	{"OpenAction", V1_1, func(c *Catalog) bool { return c.OpenAction != nil }},
	{"AA", V1_2, func(c *Catalog) bool { return c.AA != nil }},
	{"URI", V1_1, func(c *Catalog) bool { return c.URI != nil }},
	{"AcroForm", V1_2, func(c *Catalog) bool { return c.AcroForm != nil }},
	{"Metadata", V1_4, func(c *Catalog) bool { return c.Metadata != nil }},
	{"StructTreeRoot", V1_3, func(c *Catalog) bool { return c.StructTreeRoot != nil }},
	{"MarkInfo", V1_4, func(c *Catalog) bool { return c.MarkInfo != nil }},
	{"Lang", V1_4, func(c *Catalog) bool { return !c.Lang.IsRoot() }},
	{"SpiderInfo", V1_3, func(c *Catalog) bool { return c.SpiderInfo != nil }},
	{"OutputIntents", V1_4, func(c *Catalog) bool { return c.OutputIntents != nil }},
	{"PieceInfo", V1_4, func(c *Catalog) bool { return c.PieceInfo != nil }},
	{"OCProperties", V1_5, func(c *Catalog) bool { return c.OCProperties != nil }},
	{"Perms", V1_5, func(c *Catalog) bool { return c.Perms != nil }},
	{"Legal", V1_5, func(c *Catalog) bool { return c.Legal != nil }},
	{"Requirements", V1_7, func(c *Catalog) bool { return c.Requirements != nil }},
	{"Collection", V1_7, func(c *Catalog) bool { return c.Collection != nil }},
	{"NeedsRendering", V1_5, func(c *Catalog) bool { return c.NeedsRendering }},
	{"DSS", V2_0, func(c *Catalog) bool { return c.DSS != nil }},
	{"AF", V2_0, func(c *Catalog) bool { return c.AF != nil }},
	{"DPartRoot", V2_0, func(c *Catalog) bool { return c.DPartRoot != nil }},
}

// MinVersion returns the lowest PDF version in which every entry present in
// c is allowed.
func (c *Catalog) MinVersion() Version {
	v := V1_0
	for _, e := range catalogEntryVersions {
		if e.present(c) {
			v = max(v, e.version)
		}
	}
	return v
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

	for _, e := range catalogEntryVersions {
		if !e.present(c) {
			continue
		}
		if err := CheckVersion(out, "Catalog "+e.name+" entry", e.version); err != nil {
			return nil, err
		}
	}

	dict := Dict{
		"Type":  Name("Catalog"),
		"Pages": c.Pages,
	}

	if c.Version != 0 {
		// versions the library cannot write itself, such as 1.8 read
		// from an existing file, are written through unchanged
		dict["Version"] = Name(c.Version.String())
	}

	if c.Extensions != nil {
		dict["Extensions"] = c.Extensions
	}

	if c.PageLabels != nil {
		dict["PageLabels"] = c.PageLabels
	}

	if c.Names != nil {
		dict["Names"] = c.Names
	}

	if c.Dests != nil {
		dict["Dests"] = c.Dests
	}

	if c.ViewerPreferences != nil {
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
		dict["Threads"] = c.Threads
	}

	if c.OpenAction != nil {
		dict["OpenAction"] = c.OpenAction
	}

	if c.AA != nil {
		dict["AA"] = c.AA
	}

	if c.URI != nil {
		dict["URI"] = c.URI
	}

	if c.AcroForm != nil {
		dict["AcroForm"] = c.AcroForm
	}

	if c.Metadata != nil {
		ref, err := rm.Embed(c.Metadata)
		if err != nil {
			return nil, err
		}
		dict["Metadata"] = ref
	}

	if c.StructTreeRoot != nil {
		dict["StructTreeRoot"] = c.StructTreeRoot
	}

	if c.MarkInfo != nil {
		dict["MarkInfo"] = c.MarkInfo
	}

	if !c.Lang.IsRoot() {
		dict["Lang"] = Text(c.Lang.String())
	}

	if c.SpiderInfo != nil {
		dict["SpiderInfo"] = c.SpiderInfo
	}

	if c.OutputIntents != nil {
		dict["OutputIntents"] = c.OutputIntents
	}

	if c.PieceInfo != nil {
		dict["PieceInfo"] = c.PieceInfo
	}

	if c.OCProperties != nil {
		dict["OCProperties"] = c.OCProperties
	}

	if c.Perms != nil {
		dict["Perms"] = c.Perms
	}

	if c.Legal != nil {
		dict["Legal"] = c.Legal
	}

	if c.Requirements != nil {
		dict["Requirements"] = c.Requirements
	}

	if c.Collection != nil {
		dict["Collection"] = c.Collection
	}

	if c.NeedsRendering {
		dict["NeedsRendering"] = Boolean(true)
	}

	if c.DSS != nil {
		dict["DSS"] = c.DSS
	}

	if c.AF != nil {
		dict["AF"] = c.AF
	}

	if c.DPartRoot != nil {
		dict["DPartRoot"] = c.DPartRoot
	}

	return dict, nil
}
