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

package decode

import (
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
)

func decodeText(x *pdf.Extractor, path *pdf.CycleCheck, dict pdf.Dict) (*annotation.Text, error) {
	text := &annotation.Text{}

	if err := decodeCommon(x, path, &text.Common, dict); err != nil {
		return nil, err
	}

	if err := decodeMarkup(x, path, dict, &text.Markup); err != nil {
		return nil, err
	}

	if open, err := pdf.Optional(x.GetBoolean(path, dict["Open"])); err != nil {
		return nil, err
	} else {
		text.Open = bool(open)
	}

	if name, err := pdf.Optional(x.GetName(path, dict["Name"])); err != nil {
		return nil, err
	} else if name != "" {
		text.Icon = annotation.TextIcon(name)
	} else {
		text.Icon = annotation.TextIconNote
	}

	stateModel, err := pdf.Optional(pdf.GetTextString(x.R, dict["StateModel"]))
	if err != nil {
		return nil, err
	}
	switch stateModel {
	case "Marked":
		text.State = annotation.TextStateUnmarked
	case "Review":
		text.State = annotation.TextStateNone
	}

	state, err := pdf.Optional(pdf.GetTextString(x.R, dict["State"]))
	if err != nil {
		return nil, err
	}
	switch state {
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
		} else if text.Markup.User == "" {
			text.Markup.User = "unknown"
		}
	}

	return text, nil
}
