// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package decode

import (
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
)

func decodeText(c pdf.Cursor, dict pdf.Dict) (*annotation.Text, error) {
	text := &annotation.Text{}

	if err := decodeCommon(c, &text.Common, dict); err != nil {
		return nil, err
	}

	if err := decodeMarkup(c, dict, &text.Markup); err != nil {
		return nil, err
	}

	open, _, err := c.OptionalBoolean(dict["Open"])
	if err != nil {
		return nil, err
	}
	text.Open = open

	if name, err := pdf.Optional(c.Name(dict["Name"])); err != nil {
		return nil, err
	} else if name != "" {
		text.Icon = annotation.TextIcon(name)
	} else {
		text.Icon = annotation.TextIconNote
	}

	stateModel, err := pdf.Optional(c.TextString(dict["StateModel"]))
	if err != nil {
		return nil, err
	}
	switch stateModel.String() {
	case "Marked":
		text.State = annotation.TextStateUnmarked
	case "Review":
		text.State = annotation.TextStateNone
	}

	state, err := pdf.Optional(c.TextString(dict["State"]))
	if err != nil {
		return nil, err
	}
	switch state.String() {
	case "Marked":
		text.State = annotation.TextStateMarked
	case "Unmarked":
		text.State = annotation.TextStateUnmarked
	case "Accepted":
		text.State = annotation.TextStateAccepted
	case "Rejected":
		text.State = annotation.TextStateRejected
	case "Cancelled":
		text.State = annotation.TextStateCancelled
	case "Completed":
		text.State = annotation.TextStateCompleted
	case "None":
		text.State = annotation.TextStateNone
	}

	// State annotations require both InReplyTo and User fields.
	if text.State != annotation.TextStateUnknown {
		if text.Markup.InReplyTo == 0 {
			text.State = annotation.TextStateUnknown // can't fix missing reply relationship
		} else if text.Markup.User.IsZero() {
			text.Markup.User = pdf.Text("unknown")
		}
	}

	return text, nil
}
