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

// Text represents a "sticky note" text annotation.
type Text struct {
	Common
	Markup

	// Open specifies whether the annotation is initially displayed open.
	Open bool

	// Icon is the name of an icon that is used in displaying
	// the annotation. Standard names include: Comment, Key, Note, Help,
	// NewParagraph, Paragraph, Insert.
	//
	// When writing annotations, an empty Icon name can be used as a shorthand
	// [TextIconNote].
	//
	// This corresponds to the /Name entry in the PDF annotation dictionary.
	Icon TextIcon

	// State specifies the current state of the "parent" annotation denoted by
	// the [Markup.InReplyTo] field.  Text annotations which have non-zero
	// State must have the following fields set:
	//   - [Markup.InReplyTo] must indicate the annotation that this
	//     state applies to.
	//   - [Markup.User] must be set to the user who assigned the state.
	//
	// This corresponds to the /State and /StateModel entries in the PDF
	// annotation dictionary.
	State TextState
}

var _ pdf.Annotation = (*Text)(nil)

// AnnotationType returns "Text".
// This implements the [pdf.Annotation] interface.
func (t *Text) AnnotationType() pdf.Name {
	return "Text"
}

func extractText(r pdf.Getter, obj pdf.Object, singleUse bool) (*Text, error) {
	dict, err := pdf.GetDict(r, obj)
	if err != nil {
		return nil, err
	}

	text := &Text{}

	// Extract common annotation fields
	if err := extractCommon(r, &text.Common, dict, singleUse); err != nil {
		return nil, err
	}

	// Extract markup annotation fields
	if err := extractMarkup(r, dict, &text.Markup); err != nil {
		return nil, err
	}

	// Extract text-specific fields
	if open, err := pdf.GetBoolean(r, dict["Open"]); err == nil {
		text.Open = bool(open)
	}

	if name, err := pdf.GetName(r, dict["Name"]); err == nil && name != "" {
		text.Icon = TextIcon(name)
	} else {
		text.Icon = TextIconNote
	}

	stateModel, _ := pdf.GetTextString(r, dict["StateModel"])
	switch stateModel { // set default values
	case "Marked":
		text.State = TextStateUnmarked
	case "Review":
		text.State = TextStateNone
	}
	state, _ := pdf.GetTextString(r, dict["State"])
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

	// graceful fallback for invalid state annotations when reading from PDF
	if text.State != TextStateUnknown {
		if text.Markup.InReplyTo == 0 {
			text.State = TextStateUnknown // can't fix missing reply relationship
		} else if text.Markup.User == "" {
			text.Markup.User = "unknown" // preserve state with placeholder user
		}
	}

	return text, nil
}

func (t *Text) Embed(rm *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused

	dict, err := t.AsDict(rm)
	if err != nil {
		return nil, zero, err
	}

	if t.SingleUse {
		return dict, zero, nil
	}

	ref := rm.Out.Alloc()
	err = rm.Out.Put(ref, dict)
	return ref, zero, err
}

func (t *Text) AsDict(rm *pdf.ResourceManager) (pdf.Dict, error) {
	dict := pdf.Dict{
		"Subtype": pdf.Name("Text"),
	}

	if err := t.Common.fillDict(rm, dict); err != nil {
		return nil, err
	}
	if err := t.Markup.fillDict(rm, dict); err != nil {
		return nil, err
	}

	// text-specific fields
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
	TextIconComment      TextIcon = "Comment"
	TextIconKey          TextIcon = "Key"
	TextIconNote         TextIcon = "Note"
	TextIconHelp         TextIcon = "Help"
	TextIconNewParagraph TextIcon = "NewParagraph"
	TextIconParagraph    TextIcon = "Paragraph"
	TextIconInsert       TextIcon = "Insert"
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
