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

package oc

import (
	"errors"

	"golang.org/x/text/language"
	"seehuhn.de/go/pdf"
)

// PDF 2.0 sections: 8.11.4

// Usage represents an optional content usage dictionary that contains information
// describing the nature of the content controlled by an optional content group.
// This corresponds to Table 100 in the PDF specification.
type Usage struct {
	// Creator (optional) contains application-specific data associated with this group.
	Creator *UsageCreator

	// Language (optional) specifies the language of the content controlled by this group.
	Language *UsageLanguage

	// Export (optional) contains export state configuration.
	Export *UsageExport

	// Zoom (optional) specifies a range of magnifications at which content is best viewed.
	Zoom *UsageZoom

	// Print (optional) specifies content to be used when printing.
	Print *UsagePrint

	// View (optional) contains view state information.
	View *UsageView

	// User (optional) specifies users for whom this group is primarily intended.
	User *UsageUser

	// PageElement (optional) declares that the group contains a pagination artifact.
	PageElement *UsagePageElement

	// SingleUse determines if Embed returns a dictionary (true) or
	// a reference (false).
	SingleUse bool
}

var _ pdf.Embedder = (*Usage)(nil)

// ExtractUsage extracts a usage dictionary from a PDF object.
func ExtractUsage(x *pdf.Extractor, obj pdf.Object) (*Usage, error) {
	singleUse := !x.IsIndirect // capture before other x method calls

	dict, err := x.GetDict(obj)
	if err != nil {
		return nil, err
	} else if dict == nil {
		return nil, pdf.Error("missing usage dictionary")
	}

	usage := &Usage{}

	// extract CreatorInfo dictionary
	if creatorDict, err := pdf.Optional(x.GetDict(dict["CreatorInfo"])); err != nil {
		return nil, err
	} else if creatorDict != nil {
		info := &UsageCreator{}

		if creator, err := pdf.Optional(pdf.GetTextString(x.R, creatorDict["Creator"])); err != nil {
			return nil, err
		} else if creator != "" {
			info.Creator = string(creator)
		}

		if subtype, err := pdf.Optional(x.GetName(creatorDict["Subtype"])); err != nil {
			return nil, err
		} else if subtype != "" {
			info.Subtype = subtype
		}

		// collect any additional entries
		if len(creatorDict) > 2 {
			info.AdditionalInfo = make(pdf.Dict)
			for key, val := range creatorDict {
				if key != "Creator" && key != "Subtype" {
					info.AdditionalInfo[key] = val
				}
			}
		}

		usage.Creator = info
	}

	// extract Language dictionary
	if langDict, err := pdf.Optional(x.GetDict(dict["Language"])); err != nil {
		return nil, err
	} else if langDict != nil {
		if langStr, err := pdf.Optional(pdf.GetTextString(x.R, langDict["Lang"])); err != nil {
			return nil, err
		} else if langStr != "" {
			// parse language tag
			tag, err := language.Parse(string(langStr))
			if err == nil {
				info := &UsageLanguage{Lang: tag}

				if preferred, err := pdf.Optional(x.GetName(langDict["Preferred"])); err != nil {
					return nil, err
				} else if preferred == "ON" {
					info.Preferred = true
				}

				usage.Language = info
			}
			// silently ignore invalid language codes
		}
	}

	// extract Export dictionary
	if exportDict, err := pdf.Optional(x.GetDict(dict["Export"])); err != nil {
		return nil, err
	} else if exportDict != nil {
		info := &UsageExport{}

		if state, err := pdf.Optional(x.GetName(exportDict["ExportState"])); err != nil {
			return nil, err
		} else if state == "ON" {
			info.ExportState = true
		}

		usage.Export = info
	}

	// extract Zoom dictionary
	if zoomDict, err := pdf.Optional(x.GetDict(dict["Zoom"])); err != nil {
		return nil, err
	} else if zoomDict != nil {
		info := &UsageZoom{}

		if min, err := pdf.Optional(x.GetNumber(zoomDict["min"])); err != nil {
			return nil, err
		} else {
			info.Min = min
		}

		if max, err := pdf.Optional(x.GetNumber(zoomDict["max"])); err != nil {
			return nil, err
		} else if max != 0 {
			info.Max = max
		} else {
			// default to infinity
			info.Max = 1e308
		}

		usage.Zoom = info
	}

	// extract Print dictionary
	if printDict, err := pdf.Optional(x.GetDict(dict["Print"])); err != nil {
		return nil, err
	} else if printDict != nil {
		info := &UsagePrint{}

		if subtype, err := pdf.Optional(x.GetName(printDict["Subtype"])); err != nil {
			return nil, err
		} else if subtype != "" {
			info.Subtype = PrintSubtype(subtype)
		}

		if state, err := pdf.Optional(x.GetName(printDict["PrintState"])); err != nil {
			return nil, err
		} else if state == "ON" {
			info.PrintState = true
		}

		usage.Print = info
	}

	// extract View dictionary
	if viewDict, err := pdf.Optional(x.GetDict(dict["View"])); err != nil {
		return nil, err
	} else if viewDict != nil {
		info := &UsageView{}

		if state, err := pdf.Optional(x.GetName(viewDict["ViewState"])); err != nil {
			return nil, err
		} else if state == "ON" {
			info.ViewState = true
		}

		usage.View = info
	}

	// extract User dictionary
	if userDict, err := pdf.Optional(x.GetDict(dict["User"])); err != nil {
		return nil, err
	} else if userDict != nil {
		info := &UsageUser{}

		if userType, err := pdf.Optional(x.GetName(userDict["Type"])); err != nil {
			return nil, err
		} else if userType != "" {
			info.Type = UserType(userType)
		}

		// Name can be either a text string or an array of text strings
		nameObj := userDict["Name"]
		if arr, err := x.GetArray(nameObj); err == nil && arr != nil {
			info.Name = make([]string, 0, len(arr))
			for _, item := range arr {
				if str, err := pdf.Optional(pdf.GetTextString(x.R, item)); err != nil {
					return nil, err
				} else if str != "" {
					info.Name = append(info.Name, string(str))
				}
			}
		} else if str, err := pdf.Optional(pdf.GetTextString(x.R, nameObj)); err != nil {
			return nil, err
		} else if str != "" {
			info.Name = []string{string(str)}
		}

		usage.User = info
	}

	// extract PageElement dictionary
	if pageDict, err := pdf.Optional(x.GetDict(dict["PageElement"])); err != nil {
		return nil, err
	} else if pageDict != nil {
		if subtype, err := pdf.Optional(x.GetName(pageDict["Subtype"])); err != nil {
			return nil, err
		} else if subtype != "" {
			usage.PageElement = &UsagePageElement{
				Subtype: PageElement(subtype),
			}
		}
	}

	usage.SingleUse = singleUse

	return usage, nil
}

// Embed adds the usage dictionary to a PDF file.
func (u *Usage) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
	dict := pdf.Dict{}

	// embed CreatorInfo dictionary
	if u.Creator != nil {
		creatorDict := pdf.Dict{}

		if u.Creator.Creator == "" {
			return nil, errors.New("CreatorInfo.Creator is required")
		}
		creatorDict["Creator"] = pdf.TextString(u.Creator.Creator)

		if u.Creator.Subtype == "" {
			return nil, errors.New("CreatorInfo.Subtype is required")
		}
		creatorDict["Subtype"] = u.Creator.Subtype

		// add any additional entries
		for key, val := range u.Creator.AdditionalInfo {
			if key != "Creator" && key != "Subtype" {
				creatorDict[key] = val
			}
		}

		dict["CreatorInfo"] = creatorDict
	}

	// embed Language dictionary
	if u.Language != nil {
		langDict := pdf.Dict{}

		if u.Language.Lang.IsRoot() {
			return nil, errors.New("Language.Lang is required")
		}
		langDict["Lang"] = pdf.TextString(u.Language.Lang.String())

		if u.Language.Preferred {
			langDict["Preferred"] = pdf.Name("ON")
		}

		dict["Language"] = langDict
	}

	// embed Export dictionary
	if u.Export != nil {
		exportDict := pdf.Dict{}

		if u.Export.ExportState {
			exportDict["ExportState"] = pdf.Name("ON")
		} else {
			exportDict["ExportState"] = pdf.Name("OFF")
		}

		dict["Export"] = exportDict
	}

	// embed Zoom dictionary
	if u.Zoom != nil {
		zoomDict := pdf.Dict{}

		if u.Zoom.Min < 0 {
			return nil, errors.New("Zoom.Min must be non-negative")
		}
		if u.Zoom.Max <= 0 {
			return nil, errors.New("Zoom.Max must be positive")
		}
		if u.Zoom.Min > u.Zoom.Max {
			return nil, errors.New("Zoom.Min must be less than or equal to Zoom.Max")
		}
		if u.Zoom.Min != 0 {
			zoomDict["min"] = pdf.Number(u.Zoom.Min)
		}
		// only write max if it's not effectively infinity
		if u.Zoom.Max < 1e307 {
			zoomDict["max"] = pdf.Number(u.Zoom.Max)
		}

		// only include Zoom dict if it has entries
		if len(zoomDict) > 0 {
			dict["Zoom"] = zoomDict
		}
	}

	// embed Print dictionary
	if u.Print != nil {
		printDict := pdf.Dict{}

		if u.Print.Subtype != "" {
			// validate subtype
			switch u.Print.Subtype {
			case PrintSubtypeTrapping, PrintSubtypePrintersMarks, PrintSubtypeWatermark:
				printDict["Subtype"] = pdf.Name(u.Print.Subtype)
			default:
				return nil, errors.New("invalid Print.Subtype")
			}
		}

		if u.Print.PrintState {
			printDict["PrintState"] = pdf.Name("ON")
		} else {
			// only write OFF if explicitly set with a Print dictionary
			printDict["PrintState"] = pdf.Name("OFF")
		}

		dict["Print"] = printDict
	}

	// embed View dictionary
	if u.View != nil {
		viewDict := pdf.Dict{}

		if u.View.ViewState {
			viewDict["ViewState"] = pdf.Name("ON")
		} else {
			viewDict["ViewState"] = pdf.Name("OFF")
		}

		dict["View"] = viewDict
	}

	// embed User dictionary
	if u.User != nil {
		userDict := pdf.Dict{}

		if u.User.Type == "" {
			return nil, errors.New("User.Type is required")
		}
		// validate type
		switch u.User.Type {
		case UserTypeIndividual, UserTypeTitle, UserTypeOrganisation:
			userDict["Type"] = pdf.Name(u.User.Type)
		default:
			return nil, errors.New("invalid User.Type")
		}

		if len(u.User.Name) == 0 {
			return nil, errors.New("User.Name is required")
		} else if len(u.User.Name) == 1 {
			userDict["Name"] = pdf.TextString(u.User.Name[0])
		} else {
			nameArray := make(pdf.Array, len(u.User.Name))
			for i, name := range u.User.Name {
				nameArray[i] = pdf.TextString(name)
			}
			userDict["Name"] = nameArray
		}

		dict["User"] = userDict
	}

	// embed PageElement dictionary
	if u.PageElement != nil {
		pageDict := pdf.Dict{}

		if u.PageElement.Subtype == "" {
			return nil, errors.New("PageElement.Subtype is required")
		}
		// validate subtype
		switch u.PageElement.Subtype {
		case PageElementHeaderFooter, PageElementForeground, PageElementBackground, PageElementLogo:
			pageDict["Subtype"] = pdf.Name(u.PageElement.Subtype)
		default:
			return nil, errors.New("invalid PageElement.Subtype")
		}

		dict["PageElement"] = pageDict
	}

	if u.SingleUse {
		return dict, nil
	}

	ref := rm.Alloc()
	err := rm.Out().Put(ref, dict)
	if err != nil {
		return nil, err
	}
	return ref, nil
}

// UserType represents the type of user in a usage dictionary.
type UserType string

const (
	// UserTypeIndividual represents an individual user.
	UserTypeIndividual UserType = "Ind"
	// UserTypeTitle represents a title or position.
	UserTypeTitle UserType = "Ttl"
	// UserTypeOrganisation represents an organisation.
	UserTypeOrganisation UserType = "Org"
)

// PrintSubtype represents the kind of content controlled by a print usage dictionary.
type PrintSubtype string

const (
	// PrintSubtypeTrapping represents trapping content.
	PrintSubtypeTrapping PrintSubtype = "Trapping"
	// PrintSubtypePrintersMarks represents printer's marks.
	PrintSubtypePrintersMarks PrintSubtype = "PrintersMarks"
	// PrintSubtypeWatermark represents watermark content.
	PrintSubtypeWatermark PrintSubtype = "Watermark"
)

// UsageCreator contains information about the application that created an optional content group.
type UsageCreator struct {
	// Creator specifies the application that created the group.
	Creator string

	// Subtype defines the type of content controlled by the group.
	// Suggested values include "Artwork" for graphic-design or publishing applications,
	// and "Technical" for technical designs such as building plans or schematics.
	Subtype pdf.Name

	// AdditionalInfo may contain additional entries relevant to the creating application.
	AdditionalInfo pdf.Dict
}

// UsageLanguage specifies the language of the content controlled by an optional content group.
type UsageLanguage struct {
	// Lang specifies a language and possibly a locale.
	Lang language.Tag

	// Preferred indicates whether this language should be used when there is a partial
	// match but no exact match between the system language and available languages.
	Preferred bool
}

// UsageExport contains export state information.
type UsageExport struct {
	// ExportState indicates the recommended state for content when the document
	// is saved to a format that does not support optional content.
	ExportState bool
}

// UsageZoom specifies a range of magnifications at which content is best viewed.
type UsageZoom struct {
	// Min is the minimum recommended magnification factor at which the group shall be ON.
	Min float64

	// Max is the magnification factor below which the group shall be ON.
	Max float64
}

// UsagePrint specifies content to be used when printing.
type UsagePrint struct {
	// Subtype specifies the kind of content controlled by the group.
	Subtype PrintSubtype

	// PrintState indicates whether the group shall be ON or OFF when printing.
	PrintState bool
}

// UsageView contains view state information.
type UsageView struct {
	// ViewState indicates the state of the group when the document is first opened.
	ViewState bool
}

// UsageUser specifies users for whom an optional content group is primarily intended.
type UsageUser struct {
	// Type specifies whether Name refers to an individual, title/position, or organisation.
	Type UserType

	// Name represents the name(s) of the individual, position or organisation.
	Name []string
}

// UsagePageElement declares that a group contains a pagination artifact.
type UsagePageElement struct {
	// Subtype specifies the type of pagination artifact.
	Subtype PageElement
}

// PageElement represents the subtype of a page element.
type PageElement string

const (
	// PageElementHeaderFooter represents header/footer content.
	PageElementHeaderFooter PageElement = "HF"
	// PageElementForeground represents foreground image or graphics.
	PageElementForeground PageElement = "FG"
	// PageElementBackground represents background image or graphics.
	PageElementBackground PageElement = "BG"
	// PageElementLogo represents a logo.
	PageElementLogo PageElement = "L"
)
