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

package pdf

import "seehuhn.de/go/pdf/optional"

// PDF 2.0 sections: 14.3

// Info represents a PDF Document Information Dictionary.
//
// All fields in this structure are optional.  The zero value represents
// an empty information dictionary.
//
// The Document Information Dictionary is documented in section
// 14.3.3 of ISO 32000-2:2020.
type Info struct {
	Title    TextString
	Author   TextString
	Subject  TextString
	Keywords TextString

	// Creator gives the name of the application that created the original
	// document, if the document was converted to PDF from another format.
	Creator TextString

	// Producer gives the name of the application that converted the document,
	// if the document was converted to PDF from another format.
	Producer TextString

	// CreationDate gives the date and time the document was created.
	CreationDate Date

	// ModDate gives the date and time the document was most recently modified.
	ModDate Date

	// Trapped indicates whether the document has been modified to include
	// trapping information.  If not set, the trapping status is unknown.
	Trapped optional.Bool

	// Custom contains non-standard fields from the Info dictionary.
	Custom map[string]string
}

// ExtractInfo reads an Info dictionary from a PDF file.
//
// If obj is nil, the function returns nil.
func ExtractInfo(x *Extractor, obj Object) (*Info, error) {
	if obj == nil {
		return nil, nil
	}

	dict, err := x.GetDict(obj)
	if err != nil {
		return nil, err
	}
	if dict == nil {
		return nil, nil
	}

	info := &Info{}

	// text string fields
	if title, err := Optional(GetTextString(x.R, dict["Title"])); err != nil {
		return nil, err
	} else {
		info.Title = title
	}

	if author, err := Optional(GetTextString(x.R, dict["Author"])); err != nil {
		return nil, err
	} else {
		info.Author = author
	}

	if subject, err := Optional(GetTextString(x.R, dict["Subject"])); err != nil {
		return nil, err
	} else {
		info.Subject = subject
	}

	if keywords, err := Optional(GetTextString(x.R, dict["Keywords"])); err != nil {
		return nil, err
	} else {
		info.Keywords = keywords
	}

	if creator, err := Optional(GetTextString(x.R, dict["Creator"])); err != nil {
		return nil, err
	} else {
		info.Creator = creator
	}

	if producer, err := Optional(GetTextString(x.R, dict["Producer"])); err != nil {
		return nil, err
	} else {
		info.Producer = producer
	}

	// date fields
	if creationDate, err := Optional(GetDate(x.R, dict["CreationDate"])); err != nil {
		return nil, err
	} else {
		info.CreationDate = creationDate
	}

	if modDate, err := Optional(GetDate(x.R, dict["ModDate"])); err != nil {
		return nil, err
	} else {
		info.ModDate = modDate
	}

	// trapped field
	if trappedObj := dict["Trapped"]; trappedObj != nil {
		if name, err := x.GetName(trappedObj); err == nil {
			switch name {
			case "True":
				info.Trapped.Set(true)
			case "False":
				info.Trapped.Set(false)
			}
			// "Unknown" or any unrecognized value leaves Trapped unset
		}
		// ignore errors - treat as Unknown
	}

	// custom fields
	standardKeys := map[Name]bool{
		"Title": true, "Author": true, "Subject": true, "Keywords": true,
		"Creator": true, "Producer": true, "CreationDate": true, "ModDate": true,
		"Trapped": true,
	}
	for key, val := range dict {
		if standardKeys[key] {
			continue
		}
		if ts, err := GetTextString(x.R, val); err == nil && len(ts) > 0 {
			if info.Custom == nil {
				info.Custom = make(map[string]string)
			}
			info.Custom[string(key)] = string(ts)
		}
	}

	return info, nil
}

// Embed adds the Info dictionary to a PDF file.
//
// This implements the [Embedder] interface.
//
// If all fields are empty, the function returns nil.
func (info *Info) Embed(e *EmbedHelper) (Native, error) {
	if info == nil {
		return nil, nil
	}

	dict := Dict{}

	if info.Title != "" {
		dict["Title"] = info.Title
	}
	if info.Author != "" {
		dict["Author"] = info.Author
	}
	if info.Subject != "" {
		dict["Subject"] = info.Subject
	}
	if info.Keywords != "" {
		dict["Keywords"] = info.Keywords
	}
	if info.Creator != "" {
		dict["Creator"] = info.Creator
	}
	if info.Producer != "" {
		dict["Producer"] = info.Producer
	}
	if !info.CreationDate.IsZero() {
		dict["CreationDate"] = info.CreationDate.AsPDF(e.Out().GetOptions())
	}
	if !info.ModDate.IsZero() {
		dict["ModDate"] = info.ModDate.AsPDF(e.Out().GetOptions())
	}
	if trapped, ok := info.Trapped.Get(); ok {
		if err := CheckVersion(e.Out(), "Info Trapped entry", V1_3); err != nil {
			return nil, err
		}
		if trapped {
			dict["Trapped"] = Name("True")
		} else {
			dict["Trapped"] = Name("False")
		}
	}
	// unset Trapped is not written - PDF default is Unknown
	for key, val := range info.Custom {
		dict[Name(key)] = TextString(val)
	}

	if len(dict) == 0 {
		return nil, nil
	}

	ref := e.Alloc()
	if err := e.Out().Put(ref, dict); err != nil {
		return nil, err
	}
	return ref, nil
}
