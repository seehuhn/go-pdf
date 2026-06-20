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
	"seehuhn.de/go/pdf/file"
)

func decodeFileAttachment(c pdf.Cursor, dict pdf.Dict) (*annotation.FileAttachment, error) {
	fileAttachment := &annotation.FileAttachment{}

	// Extract common annotation fields
	if err := decodeCommon(c, &fileAttachment.Common, dict); err != nil {
		return nil, err
	}

	// Extract markup annotation fields
	if err := decodeMarkup(c, dict, &fileAttachment.Markup); err != nil {
		return nil, err
	}

	// Extract file attachment-specific fields
	// FS (required)
	if fs, err := pdf.DecodeOptional(c, dict["FS"], file.ExtractSpecification); err != nil {
		return nil, err
	} else if fs == nil {
		return nil, pdf.Error("file attachment annotation requires FS entry")
	} else {
		fileAttachment.FS = fs
	}

	if icon, err := pdf.Optional(c.Name(dict["Name"])); err != nil {
		return nil, err
	} else if icon != "" {
		fileAttachment.Icon = annotation.FileAttachmentIcon(icon)
	} else {
		fileAttachment.Icon = annotation.FileAttachmentIconPushPin
	}

	return fileAttachment, nil
}
