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

	// IconName (optional) is the name of an icon that is used in displaying
	// the annotation. Standard names include: Comment, Key, Note, Help,
	// NewParagraph, Paragraph, Insert. Default value: Note.
	//
	// This corresponds to the /Name entry in the PDF annotation dictionary.
	IconName Icon

	// State specifies the current state of the annotation denoted by the
	// [Markup.InReplyTo] field.
	//
	// This corresponds to the /State and /StateModel entries in the PDF
	// annotation dictionary.
	State State
}

var _ pdf.Annotation = (*Text)(nil)

// AnnotationType returns "Text".
// This implements the [pdf.Annotation] interface.
func (t *Text) AnnotationType() pdf.Name {
	return "Text"
}

func extractText(r pdf.Getter, obj pdf.Object) (*Text, error) {
	dict, err := pdf.GetDict(r, obj)
	if err != nil {
		return nil, err
	}

	text := &Text{}

	// Extract common annotation fields
	if err := extractCommon(r, dict, &text.Common); err != nil {
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
		text.IconName = Icon(name)
	} else {
		text.IconName = IconNote
	}

	stateModel, _ := pdf.GetTextString(r, dict["StateModel"])
	switch stateModel { // set default values
	case "Marked":
		text.State = StateUnmarked
	case "Review":
		text.State = StateNone
	}
	state, _ := pdf.GetTextString(r, dict["State"])
	switch state {
	case "Marked":
		text.State = StateMarked
	case "Unmarked":
		text.State = StateUnmarked
	case "Accepted":
		text.State = StateAccepted
	case "Rejected":
		text.State = StateRejected
	case "Cancelled":
		text.State = StateCancelled
	case "Completed":
		text.State = StateCompleted
	case "None":
		text.State = StateNone
	}

	// graceful fallback for invalid state annotations when reading from PDF
	if text.State != StateUnknown {
		if text.Markup.InReplyTo == 0 {
			text.State = StateUnknown // can't fix missing reply relationship
		} else if text.Markup.User == "" {
			text.Markup.User = "unknown" // preserve state with placeholder user
		}
	}

	return text, nil
}

func (t *Text) Embed(rm *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused
	ref := rm.Out.Alloc()
	if _, err := t.EmbedAt(rm, ref); err != nil {
		return nil, zero, err
	}
	return ref, zero, nil
}

func (t *Text) EmbedAt(rm *pdf.ResourceManager, ref pdf.Reference) (pdf.Unused, error) {
	var zero pdf.Unused

	dict := pdf.Dict{
		"Type":    pdf.Name("Annot"),
		"Subtype": pdf.Name("Text"),
	}

	if err := t.Common.fillDict(rm, dict); err != nil {
		return zero, err
	}
	if err := t.Markup.fillDict(rm, dict); err != nil {
		return zero, err
	}

	// text-specific fields
	if t.Open {
		dict["Open"] = pdf.Boolean(t.Open)
	}

	if t.IconName != "" && t.IconName != IconNote {
		dict["Name"] = pdf.Name(t.IconName)
	}
	if t.State != StateUnknown {
		if err := pdf.CheckVersion(rm.Out, "text annotation State entry", pdf.V1_5); err != nil {
			return zero, err
		}
		if t.Markup.User == "" {
			return zero, errors.New("missing User")
		}
		if t.Markup.InReplyTo == 0 {
			return zero, errors.New("missing InReplyTo")
		}

		switch t.State {
		case StateUnmarked:
			dict["StateModel"] = pdf.TextString("Marked")
			// dict["State"] = pdf.TextString("Unmarked")
		case StateMarked:
			dict["StateModel"] = pdf.TextString("Marked")
			dict["State"] = pdf.TextString("Marked")

		case StateAccepted:
			dict["StateModel"] = pdf.TextString("Review")
			dict["State"] = pdf.TextString("Accepted")
		case StateRejected:
			dict["StateModel"] = pdf.TextString("Review")
			dict["State"] = pdf.TextString("Rejected")
		case StateCancelled:
			dict["StateModel"] = pdf.TextString("Review")
			dict["State"] = pdf.TextString("Cancelled")
		case StateCompleted:
			dict["StateModel"] = pdf.TextString("Review")
			dict["State"] = pdf.TextString("Completed")
		case StateNone:
			dict["StateModel"] = pdf.TextString("Review")
			// dict["State"] = pdf.TextString("None")
		}
	}

	err := rm.Out.Put(ref, dict)
	if err != nil {
		return zero, err
	}

	return zero, nil
}
