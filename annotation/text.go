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

import "seehuhn.de/go/pdf"

// Text represents a "sticky note" text annotation.
type Text struct {
	Common
	Markup

	// Open specifies whether the annotation shall initially be displayed open.
	Open bool

	// Name (optional) is the name of an icon that shall be used in displaying
	// the annotation. Standard names include: Comment, Key, Note, Help,
	// NewParagraph, Paragraph, Insert. Default value: Note.
	Name pdf.Name

	// State (optional; PDF 1.5) is the state to which the original annotation
	// shall be set. Default: "Unmarked" if StateModel is "Marked"; "None" if
	// StateModel is "Review".
	State string

	// StateModel (required if State is present, otherwise optional; PDF 1.5)
	// is the state model corresponding to State.
	StateModel string
}

var _ pdf.Annotation = (*Text)(nil)

// AnnotationType returns "Text".
// This implements the [pdf.Annotation] interface.
func (t *Text) AnnotationType() string {
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

	if name, err := pdf.GetName(r, dict["Name"]); err == nil {
		text.Name = name
	}

	if state, err := pdf.GetTextString(r, dict["State"]); err == nil {
		text.State = string(state)
	}

	if stateModel, err := pdf.GetTextString(r, dict["StateModel"]); err == nil {
		text.StateModel = string(stateModel)
	}

	return text, nil
}

func (t *Text) Embed(rm *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused

	dict := pdf.Dict{
		"Type":    pdf.Name("Annot"),
		"Subtype": pdf.Name("Text"),
	}

	// Add common annotation fields
	if err := t.Common.fillDict(rm, dict); err != nil {
		return nil, zero, err
	}

	// Add markup annotation fields
	if err := t.Markup.fillDict(rm, dict); err != nil {
		return nil, zero, err
	}

	// Add text-specific fields
	if t.Open {
		dict["Open"] = pdf.Boolean(t.Open)
	}

	if t.Name != "" {
		dict["Name"] = t.Name
	}

	if t.State != "" {
		if err := pdf.CheckVersion(rm.Out, "text annotation State entry", pdf.V1_5); err != nil {
			return nil, zero, err
		}
		dict["State"] = pdf.TextString(t.State)
	}

	if t.StateModel != "" {
		if err := pdf.CheckVersion(rm.Out, "text annotation StateModel entry", pdf.V1_5); err != nil {
			return nil, zero, err
		}
		dict["StateModel"] = pdf.TextString(t.StateModel)
	}

	return dict, zero, nil
}
