package pdf

import (
	"errors"

	"golang.org/x/text/language"
)

// Catalog represents a PDF Document Catalog.  The only required field in this
// structure is Pages, which specifies the root of the page tree.
// This struct can be used with the [DecodeDict] and [AsDict] functions.
//
// The Document Catalog is documented in section 7.7.2 of PDF 32000-1:2008.
type Catalog struct {
	_ struct{} `pdf:"Type=Catalog"`

	// Version (optional, PDF 1.4) specifies the PDF version this document
	// conforms to if later than the version in the file header.
	Version Version `pdf:"optional"`

	// Extensions (optional, PDF 1.4) contains developer extensions information
	// for extensions that occur in this document.
	Extensions Object `pdf:"optional"`

	// Pages is the root of the document's page tree.
	Pages Reference

	// PageLabels (optional, PDF 1.3) defines the page labeling for the
	// document as a number tree where keys are page indices and values are
	// page label dictionaries.
	PageLabels Object `pdf:"optional"`

	// Names (optional, PDF 1.2) is the document's name dictionary.
	Names Object `pdf:"optional"`

	// Dests (optional, PDF 1.1) contains a dictionary of names and
	// corresponding destinations.
	Dests Object `pdf:"optional"`

	// ViewerPreferences (optional, PDF 1.2) specifies how the document should
	// be displayed on screen.
	ViewerPreferences Object `pdf:"optional"`

	// PageLayout (optional) specifies the page layout to use when the document
	// is opened. Valid values are SinglePage, OneColumn, TwoColumnLeft,
	// TwoColumnRight, TwoPageLeft, TwoPageRight.
	PageLayout Name `pdf:"optional"`

	// PageMode (optional) specifies how the document should be displayed when
	// opened. Valid values are UseNone, UseOutlines, UseThumbs, FullScreen,
	// UseOC, UseAttachments.
	PageMode Name `pdf:"optional"`

	// Outlines (optional) is the root of the document's outline hierarchy.
	Outlines Reference `pdf:"optional"`

	// Threads (optional, PDF 1.1) contains an array of thread dictionaries
	// representing the document's article threads.
	Threads Reference `pdf:"optional"`

	// OpenAction (optional, PDF 1.1) specifies a destination to display or
	// action to perform when the document is opened.
	OpenAction Object `pdf:"optional"`

	// AA (optional, PDF 1.2) defines additional actions to take in response to
	// various trigger events affecting the document.
	AA Object `pdf:"optional"`

	// URI (optional, PDF 1.1) contains document-level information for URI
	// actions.
	URI Object `pdf:"optional"`

	// AcroForm (optional, PDF 1.2) is the document's interactive form
	// dictionary.
	AcroForm Object `pdf:"optional"`

	// Metadata (optional, PDF 1.4) contains metadata for the document.
	Metadata Reference `pdf:"optional"`

	// StructTreeRoot (optional, PDF 1.3) is the document's structure tree root
	// dictionary.
	StructTreeRoot Object `pdf:"optional"`

	// MarkInfo (optional, PDF 1.4) contains information about the document's
	// usage of tagged PDF conventions.
	MarkInfo Object `pdf:"optional"`

	// Lang (optional, PDF 1.4) specifies the natural language for all text in
	// the document.
	Lang language.Tag `pdf:"optional"`

	// SpiderInfo (optional, PDF 1.3) contains Web Capture information and state.
	SpiderInfo Object `pdf:"optional"`

	// OutputIntents (optional, PDF 1.4) specifies the color characteristics of
	// output devices on which the document might be rendered.
	OutputIntents Object `pdf:"optional"`

	// PieceInfo (optional, PDF 1.4) is a page-piece dictionary associated with
	// the document.
	PieceInfo Object `pdf:"optional"`

	// OCProperties (optional, PDF 1.5) contains the document's optional
	// content properties. Required if the document contains optional content.
	OCProperties Object `pdf:"optional"`

	// Perms (optional, PDF 1.5) specifies user access permissions for the
	// document.
	Perms Object `pdf:"optional"`

	// Legal (optional, PDF 1.5) contains attestations regarding the content of
	// the PDF document as it relates to the legality of digital signatures.
	Legal Object `pdf:"optional"`

	// Requirements (optional, PDF 1.7) contains an array of requirement
	// dictionaries that represent requirements for the document.
	Requirements Object `pdf:"optional"`

	// Collection (optional, PDF 1.7) enhances the presentation of file
	// attachments stored in the PDF document.
	Collection Object `pdf:"optional"`

	// NeedsRendering (optional, deprecated in PDF 2.0) specifies whether the
	// document should be regenerated when first opened. Used for XFA forms.
	NeedsRendering bool `pdf:"optional"`

	// DSS (optional, PDF 2.0) contains document-wide security information.
	DSS Object `pdf:"optional"`

	// AF (optional, PDF 2.0) contains an array of file specification
	// dictionaries denoting the associated files for this PDF document.
	AF Object `pdf:"optional"`

	// DPartRoot (optional, PDF 2.0) describes the document parts hierarchy for
	// this PDF document.
	DPartRoot Object `pdf:"optional"`
}

func ExtractCatalog(r Getter, obj Object) (*Catalog, error) {
	dict, err := GetDictTyped(r, obj, "Catalog")
	if err != nil {
		return nil, err
	}
	if dict == nil {
		return nil, &MalformedFileError{
			Err: errors.New("catalog dictionary is missing"),
		}
	}

	// Extract required Pages field
	pagesObj := dict["Pages"]
	if pagesObj == nil {
		return nil, &MalformedFileError{
			Err: errors.New("required field Pages is missing"),
		}
	}

	// Try to get Pages as Reference, but be permissive
	var pages Reference
	if ref, ok := pagesObj.(Reference); ok {
		pages = ref
	} else {
		// For malformed files, try to proceed anyway
		pages = 0
	}

	// Extract optional fields
	pageLayout, _ := GetName(r, dict["PageLayout"])
	pageMode, _ := GetName(r, dict["PageMode"])

	var outlines Reference
	if ref, ok := dict["Outlines"].(Reference); ok {
		outlines = ref
	}

	var threads Reference
	if ref, ok := dict["Threads"].(Reference); ok {
		threads = ref
	}

	var metadata Reference
	if ref, ok := dict["Metadata"].(Reference); ok {
		metadata = ref
	}

	// Extract Lang field
	var lang language.Tag
	if dict["Lang"] != nil {
		langStr, err := GetTextString(r, dict["Lang"])
		if err == nil && langStr != "" {
			lang, _ = language.Parse(string(langStr))
		}
	}

	// Extract NeedsRendering
	needsRendering, _ := GetBoolean(r, dict["NeedsRendering"])

	return &Catalog{
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
