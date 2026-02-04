// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2026  Jochen Voss <voss@seehuhn.de>
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

package oc

import (
	"errors"

	"seehuhn.de/go/pdf"
)

// PDF 2.0 sections: 8.11.4.4

// Event specifies when usage application settings should be applied.
type Event pdf.Name

const (
	// EventView applies when the document is being viewed interactively.
	EventView Event = "View"

	// EventPrint applies when the document is being printed.
	EventPrint Event = "Print"

	// EventExport applies when the document is being exported.
	EventExport Event = "Export"
)

// Category specifies which usage dictionary entry to consult.
type Category pdf.Name

const (
	// CategoryCreatorInfo specifies the CreatorInfo usage dictionary entry.
	CategoryCreatorInfo Category = "CreatorInfo"

	// CategoryLanguage specifies the Language usage dictionary entry.
	CategoryLanguage Category = "Language"

	// CategoryExport specifies the Export usage dictionary entry.
	CategoryExport Category = "Export"

	// CategoryZoom specifies the Zoom usage dictionary entry.
	CategoryZoom Category = "Zoom"

	// CategoryPrint specifies the Print usage dictionary entry.
	CategoryPrint Category = "Print"

	// CategoryView specifies the View usage dictionary entry.
	CategoryView Category = "View"

	// CategoryUser specifies the User usage dictionary entry.
	CategoryUser Category = "User"

	// CategoryPageElement specifies the PageElement usage dictionary entry.
	CategoryPageElement Category = "PageElement"
)

// knownCategories is the set of valid Category values.
var knownCategories = map[Category]bool{
	CategoryCreatorInfo: true,
	CategoryLanguage:    true,
	CategoryExport:      true,
	CategoryZoom:        true,
	CategoryPrint:       true,
	CategoryView:        true,
	CategoryUser:        true,
	CategoryPageElement: true,
}

// UsageApplication specifies rules for automatically managing optional content
// group states based on external factors such as zoom level, language, or
// whether the document is being viewed, printed, or exported.
// This corresponds to Table 101 in the PDF specification.
type UsageApplication struct {
	// Event specifies when to apply: View, Print, or Export.
	Event Event

	// OCGs (optional) lists the optional content groups to manage.
	OCGs []*Group

	// Category lists which usage dictionary entries to consult.
	Category []Category

	// SingleUse determines if Embed returns a dictionary (true) or
	// a reference (false). Typically this should be true since usage
	// application dictionaries are usually embedded directly in the AS array.
	SingleUse bool
}

var _ pdf.Embedder = (*UsageApplication)(nil)

// ExtractUsageApplication extracts a usage application dictionary from a PDF object.
func ExtractUsageApplication(x *pdf.Extractor, obj pdf.Object) (*UsageApplication, error) {
	singleUse := !x.IsIndirect // capture before other x method calls

	r := x.R
	dict, err := pdf.GetDict(r, obj)
	if err != nil {
		return nil, err
	} else if dict == nil {
		return nil, pdf.Error("missing usage application dictionary")
	}

	ua := &UsageApplication{}

	// extract Event (required)
	if event, err := pdf.Optional(pdf.GetName(r, dict["Event"])); err != nil {
		return nil, err
	} else {
		switch Event(event) {
		case EventView, EventPrint, EventExport:
			ua.Event = Event(event)
		default:
			// default to View for unknown/missing values
			ua.Event = EventView
		}
	}

	// extract OCGs (optional, normalized to nil if empty)
	if arr, err := pdf.Optional(pdf.GetArray(r, dict["OCGs"])); err != nil {
		return nil, err
	} else if len(arr) > 0 {
		var ocgs []*Group
		for _, item := range arr {
			if group, err := pdf.ExtractorGetOptional(x, item, ExtractGroup); err != nil {
				return nil, err
			} else if group != nil {
				ocgs = append(ocgs, group)
			}
		}
		if len(ocgs) > 0 {
			ua.OCGs = ocgs[:len(ocgs):len(ocgs)]
		}
	}

	// extract Category (required)
	arr, err := pdf.Optional(pdf.GetArray(r, dict["Category"]))
	if err != nil {
		return nil, err
	}
	for _, item := range arr {
		if name, err := pdf.Optional(pdf.GetName(r, item)); err != nil {
			return nil, err
		} else if name != "" {
			cat := Category(name)
			if knownCategories[cat] {
				ua.Category = append(ua.Category, cat)
			}
		}
	}

	// error if no recognized categories remain
	if len(ua.Category) == 0 {
		return nil, pdf.Error("usage application dictionary has no recognized categories")
	}

	ua.SingleUse = singleUse

	return ua, nil
}

// Embed adds the usage application dictionary to a PDF file.
func (ua *UsageApplication) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out(), "usage application dictionary", pdf.V1_5); err != nil {
		return nil, err
	}

	// validate Event
	switch ua.Event {
	case EventView, EventPrint, EventExport:
		// valid
	default:
		return nil, errors.New("invalid Event value: must be View, Print, or Export")
	}

	// validate Category
	if len(ua.Category) == 0 {
		return nil, errors.New("Category is required and must be non-empty")
	}
	for _, cat := range ua.Category {
		if !knownCategories[cat] {
			return nil, errors.New("invalid Category value")
		}
	}

	dict := pdf.Dict{
		"Event": pdf.Name(ua.Event),
	}

	// embed OCGs
	if len(ua.OCGs) > 0 {
		ocgArray := make(pdf.Array, len(ua.OCGs))
		for i, group := range ua.OCGs {
			ocgObj, err := rm.Embed(group)
			if err != nil {
				return nil, err
			}
			ocgArray[i] = ocgObj
		}
		dict["OCGs"] = ocgArray
	}

	// embed Category
	catArray := make(pdf.Array, len(ua.Category))
	for i, cat := range ua.Category {
		catArray[i] = pdf.Name(cat)
	}
	dict["Category"] = catArray

	if ua.SingleUse {
		return dict, nil
	}

	ref := rm.Alloc()
	err := rm.Out().Put(ref, dict)
	if err != nil {
		return nil, err
	}
	return ref, nil
}
