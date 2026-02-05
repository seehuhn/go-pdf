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

package annotation

import (
	"errors"

	"seehuhn.de/go/pdf"
)

// PDF 2.0 sections: 12.5.2 12.5.6.2 12.5.6.4

// Text is used to provide editorial notes, comments, or other textual feedback
// on a PDF document. Text annotations are displayed as icons on the page,
// which can be clicked to reveal the associated text content in a pop-up
// window.
//
// The background color of the icon can be specified using the [Common.Color]
// field.  The default color is implementation dependent.
type Text struct {
	Common
	Markup

	// Open specifies whether the pop-up window should be initially open.
	Open bool

	// Icon is the name of an icon that is used in displaying the annotation.
	// The standard icon names are Comment, Key, Note, Help, NewParagraph,
	// Paragraph, Insert.  Viewers may support additional, application-specific
	// names.
	//
	// When writing annotations, an empty Icon name can be used as a shorthand
	// for [TextIconNote].
	//
	// This corresponds to the /Name entry in the PDF annotation dictionary.
	Icon TextIcon

	// State (optional) specifies the current state of the "parent" annotation
	// denoted by the [Markup.InReplyTo] field.  Text annotations with
	// non-zero State must have the following fields set:
	//
	//   - [Markup.InReplyTo] must indicate the annotation that this
	//     state applies to.
	//   - [Markup.User] must be set to the user who assigned the state.
	//
	// This corresponds to the /State and /StateModel entries in the PDF
	// annotation dictionary.
	State TextState
}

var _ Annotation = (*Text)(nil)

// AnnotationType returns "Text".
// This implements the [Annotation] interface.
func (t *Text) AnnotationType() pdf.Name {
	return "Text"
}

func decodeText(x *pdf.Extractor, dict pdf.Dict) (*Text, error) {
	text := &Text{}

	if err := decodeCommon(x, &text.Common, dict); err != nil {
		return nil, err
	}

	if err := decodeMarkup(x, dict, &text.Markup); err != nil {
		return nil, err
	}

	if open, err := pdf.Optional(x.GetBoolean(dict["Open"])); err != nil {
		return nil, err
	} else {
		text.Open = bool(open)
	}

	if name, err := pdf.Optional(x.GetName(dict["Name"])); err != nil {
		return nil, err
	} else if name != "" {
		text.Icon = TextIcon(name)
	} else {
		text.Icon = TextIconNote
	}

	stateModel, err := pdf.Optional(pdf.GetTextString(x.R, dict["StateModel"]))
	if err != nil {
		return nil, err
	}
	switch stateModel {
	case "Marked":
		text.State = TextStateUnmarked
	case "Review":
		text.State = TextStateNone
	}

	state, err := pdf.Optional(pdf.GetTextString(x.R, dict["State"]))
	if err != nil {
		return nil, err
	}
	switch state {
	case "Marked":
		text.State = TextStateMarked
	case "Unmarked":
		text.State = TextStateUnmarked
	case "Accepted":
		text.State = TextStateAccepted
	case "Rejected":
		text.State = TextStateRejected
	case "Cancelled":
		text.State = TextStateCancelled
	case "Completed":
		text.State = TextStateCompleted
	case "None":
		text.State = TextStateNone
	}

	// State annotations require both InReplyTo and User fields.
	if text.State != TextStateUnknown {
		if text.Markup.InReplyTo == 0 {
			text.State = TextStateUnknown // can't fix missing reply relationship
		} else if text.Markup.User == "" {
			text.Markup.User = "unknown"
		}
	}

	return text, nil
}

func (t *Text) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	dict := pdf.Dict{
		"Subtype": pdf.Name("Text"),
	}

	if err := t.Common.fillDict(rm, dict, isMarkup(t), false); err != nil {
		return nil, err
	}
	if err := t.Markup.fillDict(rm, dict); err != nil {
		return nil, err
	}

	if t.Open {
		dict["Open"] = pdf.Boolean(t.Open)
	}

	if t.Icon != "" && t.Icon != TextIconNote {
		dict["Name"] = pdf.Name(t.Icon)
	}
	if t.State != TextStateUnknown {
		if err := pdf.CheckVersion(rm.Out, "text annotation State entry", pdf.V1_5); err != nil {
			return nil, err
		}
		if t.Markup.User == "" {
			return nil, errors.New("missing User")
		}
		if t.Markup.InReplyTo == 0 {
			return nil, errors.New("missing InReplyTo")
		}

		switch t.State {
		case TextStateUnmarked:
			dict["StateModel"] = pdf.TextString("Marked")
			// dict["State"] = pdf.TextString("Unmarked")
		case TextStateMarked:
			dict["StateModel"] = pdf.TextString("Marked")
			dict["State"] = pdf.TextString("Marked")

		case TextStateAccepted:
			dict["StateModel"] = pdf.TextString("Review")
			dict["State"] = pdf.TextString("Accepted")
		case TextStateRejected:
			dict["StateModel"] = pdf.TextString("Review")
			dict["State"] = pdf.TextString("Rejected")
		case TextStateCancelled:
			dict["StateModel"] = pdf.TextString("Review")
			dict["State"] = pdf.TextString("Cancelled")
		case TextStateCompleted:
			dict["StateModel"] = pdf.TextString("Review")
			dict["State"] = pdf.TextString("Completed")
		case TextStateNone:
			dict["StateModel"] = pdf.TextString("Review")
			// dict["State"] = pdf.TextString("None")
		}
	}

	return dict, nil
}

// TextIcon represents the name of an icon used to represent a text annotation.
// The standard names defined by the PDF specification are provided as constants.
// Other names may be used, but support is viewer dependent.
type TextIcon pdf.Name

// Standard PDF icon names for text annotations.
const (
	// TextIconComment represents general feedback or discussion points.
	// Typically appears as a speech bubble icon in PDF viewers.
	TextIconComment TextIcon = "Comment"

	// TextIconKey marks important or critical information requiring special attention.
	// Typically appears as a key symbol or star icon in PDF viewers.
	TextIconKey TextIcon = "Key"

	// TextIconNote provides explanatory text, clarifications, or additional information.
	// Typically appears as a sticky note (post-it) icon in PDF viewers.
	TextIconNote TextIcon = "Note"

	// TextIconHelp indicates questions, requests for clarification, or help-related content.
	// Typically appears as a question mark or help "i" icon in PDF viewers.
	TextIconHelp TextIcon = "Help"

	// TextIconNewParagraph indicates where a paragraph break should be inserted.
	// Typically appears as a pilcrow (¶) symbol in PDF viewers.
	TextIconNewParagraph TextIcon = "NewParagraph"

	// TextIconParagraph provides comments about existing paragraph structure or content.
	// Typically appears as a pilcrow (¶) symbol or text block icon in PDF viewers.
	TextIconParagraph TextIcon = "Paragraph"

	// TextIconInsert indicates where content should be added or inserted.
	// Typically appears as a caret (^) or insertion cursor icon in PDF viewers.
	TextIconInsert TextIcon = "Insert"
)

// TextState represents a PDF annotation state.
type TextState int

// State represents the valid values of a [TextState] variable.
const (
	// TextStateUnknown indicates that no /State or /StateModel field are present.
	TextStateUnknown TextState = iota

	// Values following the "Marked" state model.
	TextStateUnmarked
	TextStateMarked

	// Values following the "Review" state model.
	TextStateAccepted
	TextStateRejected
	TextStateCancelled
	TextStateCompleted
	TextStateNone
)
