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

package collection

import (
	"fmt"
	"time"

	"seehuhn.de/go/pdf"
)

// PDF 2.0 sections: 7.11.6

// ItemDict represents a collection item dictionary that contains data described
// by the collection schema dictionary for a particular file in a collection.
type ItemDict struct {
	// Data provides the data corresponding to the related fields in the
	// collection dictionary.
	Data map[pdf.Name]ItemValue

	// SingleUse determines if Embed returns a dictionary (true) or
	// a reference (false).
	SingleUse bool
}

// ItemValue represents an entry in a collection item dictionary.
type ItemValue struct {
	// Val is the value associated with the key in the item dictionary.
	// Values must be one of the following types:
	//   - string
	//   - time.Time
	//   - int64
	//   - float64
	Val any

	// Prefix is an optional prefix to be applied when the value is displayed.
	// The prefix string is ignored when sorting items.
	Prefix string
}

// ExtractItemDict extracts a collection item dictionary from a PDF object.
func ExtractItemDict(x *pdf.Extractor, obj pdf.Object) (*ItemDict, error) {
	dict, err := pdf.GetDictTyped(x.R, obj, "CollectionItem")
	if err != nil {
		return nil, err
	} else if dict == nil {
		return nil, pdf.Error("missing collection item dictionary")
	}

	item := &ItemDict{
		Data: make(map[pdf.Name]ItemValue),
	}

	// Process all entries except Type
	for key, value := range dict {
		if key == "Type" {
			continue
		}

		itemValue, err := pdf.Optional(extractItemValue(x, value))
		if err != nil {
			return nil, err
		} else if itemValue != nil {
			item.Data[key] = *itemValue
		}
	}

	return item, nil
}

// extractItemValue extracts a single item value from a PDF object.
func extractItemValue(x *pdf.Extractor, obj pdf.Object) (*ItemValue, error) {
	resolved, err := pdf.Resolve(x.R, obj)
	if err != nil {
		return nil, err
	}
	if resolved == nil {
		return nil, nil
	}

	var data pdf.Object
	var prefix string
	var val any

	// Check if it's a dictionary (subitem)
	if dict, ok := resolved.(pdf.Dict); ok {
		if err := pdf.CheckDictType(x.R, dict, "CollectionSubitem"); err != nil {
			return nil, nil // Not a subitem dictionary, ignore
		}

		// Extract prefix from subitem dictionary
		if prefixObj := dict["P"]; prefixObj != nil {
			if prefixStr, err := pdf.Optional(pdf.GetTextString(x.R, prefixObj)); err != nil {
				return nil, err
			} else {
				prefix = string(prefixStr)
			}
		}

		// Use the D entry as the data to process
		data = dict["D"]
		if data == nil {
			return nil, nil
		}

		// Resolve the D entry
		if resolved, err := pdf.Resolve(x.R, data); err != nil {
			return nil, err
		} else {
			data = resolved
		}
	} else {
		// Direct value, not a subitem dictionary
		data = resolved
	}

	if data == nil {
		return nil, nil
	}

	// Type switch on the data value
	switch v := data.(type) {
	case pdf.String:
		// Try as date first, then as text string
		if dateObj, err := pdf.GetDate(x.R, v); err == nil && !dateObj.IsZero() {
			val = time.Time(dateObj)
		} else {
			val = string(v)
		}

	case pdf.Integer:
		val = int64(v)

	case pdf.Real:
		val = float64(v)

	case pdf.Number:
		val = float64(v)

	default:
		return nil, nil
	}

	return &ItemValue{Val: val, Prefix: prefix}, nil
}

// Embed converts the collection item dictionary to a PDF object.
func (item *ItemDict) Embed(rm *pdf.EmbedHelper) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused

	if err := pdf.CheckVersion(rm.Out(), "collection item dictionary", pdf.V1_7); err != nil {
		return nil, zero, err
	}

	// Check for reserved Type key
	if _, hasType := item.Data["Type"]; hasType {
		return nil, zero, fmt.Errorf("collection item data cannot contain 'Type' key")
	}

	dict := pdf.Dict{}

	// Add Type field if requested
	if rm.Out().GetOptions().HasAny(pdf.OptDictTypes) {
		dict["Type"] = pdf.Name("CollectionItem")
	}

	// Add data entries
	for key, value := range item.Data {
		pdfObj, err := embedItemValue(rm, value)
		if err != nil {
			return nil, zero, err
		}
		if pdfObj != nil {
			dict[key] = pdfObj
		}
	}

	if item.SingleUse {
		return dict, zero, nil
	}

	ref := rm.Alloc()
	err := rm.Out().Put(ref, dict)
	if err != nil {
		return nil, zero, err
	}

	return ref, zero, nil
}

// embedItemValue converts an ItemValue to a PDF object.
func embedItemValue(rm *pdf.EmbedHelper, value ItemValue) (pdf.Object, error) {
	var pdfVal pdf.Object

	// Convert the Go value to PDF object
	switch v := value.Val.(type) {
	case string:
		pdfVal = pdf.TextString(v)

	case time.Time:
		pdfVal = pdf.Date(v).AsPDF(rm.Out().GetOptions())

	case int64:
		pdfVal = pdf.Integer(v)

	case float64:
		pdfVal = pdf.Number(v)

	default:
		return nil, fmt.Errorf("unsupported collection item value type: %T", value.Val)
	}

	// If no prefix, return the direct value
	if value.Prefix == "" {
		return pdfVal, nil
	}

	// Create subitem dictionary with prefix
	subitemDict := pdf.Dict{
		"D": pdfVal,
		"P": pdf.TextString(value.Prefix),
	}

	if rm.Out().GetOptions().HasAny(pdf.OptDictTypes) {
		subitemDict["Type"] = pdf.Name("CollectionSubitem")
	}

	return subitemDict, nil
}
