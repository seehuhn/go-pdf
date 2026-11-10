// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package decode

import (
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/file"
)

func decodeFileAttachment(c pdf.Cursor, dict pdf.Dict) (*annotation.FileAttachment, error) {
	fileAttachment := &annotation.FileAttachment{}

	if err := decodeCommon(c, &fileAttachment.Common, dict); err != nil {
		return nil, err
	}

	// Extract markup annotation fields
	if err := decodeMarkup(c, dict, &fileAttachment.Markup); err != nil {
		return nil, err
	}

	// Extract file attachment-specific fields
	// FS (required)
	if fs, err := c.DecodeOptional(dict["FS"], file.ExtractSpecification); err != nil {
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
