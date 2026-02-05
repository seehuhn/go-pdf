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
	"seehuhn.de/go/pdf/file"
)

// PDF 2.0 sections: 12.5.2 12.5.6.2 12.5.6.15

// FileAttachment represents a file attachment annotation that contains a
// reference to a file, which typically is embedded in the PDF file.
// Activating the annotation extracts the embedded file and gives the user
// an opportunity to view it or store it in the file system.
type FileAttachment struct {
	Common
	Markup

	// Icon is the name of an icon that is used in displaying the annotation.
	// The standard icon names are Graph, PushPin, Paperclip, and Tag.
	//
	// When writing annotations, an empty Icon name can be used as a shorthand
	// for [FileAttachmentIconPushPin].
	//
	// This corresponds to the /Name entry in the PDF annotation dictionary.
	Icon FileAttachmentIcon

	// FS (required) is the file specification associated with this annotation.
	// This typically references an embedded file stream.
	FS *file.Specification
}

var _ Annotation = (*FileAttachment)(nil)

// AnnotationType returns "FileAttachment".
// This implements the [Annotation] interface.
func (f *FileAttachment) AnnotationType() pdf.Name {
	return "FileAttachment"
}

func decodeFileAttachment(x *pdf.Extractor, dict pdf.Dict) (*FileAttachment, error) {
	r := x.R
	fileAttachment := &FileAttachment{}

	// Extract common annotation fields
	if err := decodeCommon(x, &fileAttachment.Common, dict); err != nil {
		return nil, err
	}

	// Extract markup annotation fields
	if err := decodeMarkup(x, dict, &fileAttachment.Markup); err != nil {
		return nil, err
	}

	// Extract file attachment-specific fields
	// FS (required)
	if fs, err := pdf.ExtractorGetOptional(x, dict["FS"], file.ExtractSpecification); err != nil {
		return nil, err
	} else if fs == nil {
		return nil, errors.New("file attachment annotation requires FS entry")
	} else {
		fileAttachment.FS = fs
	}

	if icon, err := pdf.Optional(pdf.GetName(r, dict["Name"])); err != nil {
		return nil, err
	} else if icon != "" {
		fileAttachment.Icon = FileAttachmentIcon(icon)
	} else {
		fileAttachment.Icon = FileAttachmentIconPushPin
	}

	return fileAttachment, nil
}

func (f *FileAttachment) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out, "file attachment annotation", pdf.V1_3); err != nil {
		return nil, err
	}

	dict := pdf.Dict{
		"Subtype": pdf.Name("FileAttachment"),
	}

	// Add common annotation fields
	if err := f.Common.fillDict(rm, dict, isMarkup(f), false); err != nil {
		return nil, err
	}

	// Add markup annotation fields
	if err := f.Markup.fillDict(rm, dict); err != nil {
		return nil, err
	}

	// Add file attachment-specific fields
	// FS (required)
	if f.FS == nil {
		return nil, errors.New("file attachment annotation requires FS entry")
	}
	fsObj, err := rm.Embed(f.FS)
	if err != nil {
		return nil, err
	}
	dict["FS"] = fsObj

	if f.Icon != "" && f.Icon != FileAttachmentIconPushPin {
		dict["Name"] = pdf.Name(f.Icon)
	}

	return dict, nil
}

// FileAttachmentIcon represents the name of an icon used in displaying a file attachment annotation.
// The standard names defined by the PDF specification are provided as constants.
// Other names may be used, but support is viewer dependent.
type FileAttachmentIcon pdf.Name

const (
	FileAttachmentIconPushPin   FileAttachmentIcon = "PushPin"
	FileAttachmentIconGraph     FileAttachmentIcon = "Graph"
	FileAttachmentIconPaperclip FileAttachmentIcon = "Paperclip"
	FileAttachmentIconTag       FileAttachmentIcon = "Tag"
)
