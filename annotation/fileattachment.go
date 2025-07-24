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

// FileAttachment represents a file attachment annotation that contains a
// reference to a file, which typically is embedded in the PDF file.
// Activating the annotation extracts the embedded file and gives the user
// an opportunity to view it or store it in the file system.
type FileAttachment struct {
	Common
	Markup

	// FS (required) is the file specification associated with this annotation.
	// This typically references an embedded file stream.
	FS pdf.Reference

	// Name (optional) is the name of an icon that is used in displaying
	// the annotation. Standard names include:
	// Graph, PushPin, Paperclip, Tag
	// Default value: "PushPin"
	Name pdf.Name
}

var _ Annotation = (*FileAttachment)(nil)

// AnnotationType returns "FileAttachment".
// This implements the [Annotation] interface.
func (f *FileAttachment) AnnotationType() pdf.Name {
	return "FileAttachment"
}

func extractFileAttachment(r pdf.Getter, dict pdf.Dict, singleUse bool) (*FileAttachment, error) {
	fileAttachment := &FileAttachment{}

	// Extract common annotation fields
	if err := extractCommon(r, &fileAttachment.Common, dict, singleUse); err != nil {
		return nil, err
	}

	// Extract markup annotation fields
	if err := extractMarkup(r, dict, &fileAttachment.Markup); err != nil {
		return nil, err
	}

	// Extract file attachment-specific fields
	// FS (required)
	if fs, ok := dict["FS"].(pdf.Reference); ok {
		fileAttachment.FS = fs
	}

	// Name (optional)
	if name, err := pdf.GetName(r, dict["Name"]); err == nil && name != "" {
		fileAttachment.Name = name
	}

	return fileAttachment, nil
}

func (f *FileAttachment) Embed(rm *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
	dict, err := f.AsDict(rm)
	if err != nil {
		return nil, pdf.Unused{}, err
	}

	if f.SingleUse {
		return dict, pdf.Unused{}, nil
	}

	ref := rm.Out.Alloc()
	err = rm.Out.Put(ref, dict)
	return ref, pdf.Unused{}, err
}

func (f *FileAttachment) AsDict(rm *pdf.ResourceManager) (pdf.Dict, error) {
	if err := pdf.CheckVersion(rm.Out, "file attachment annotation", pdf.V1_3); err != nil {
		return nil, err
	}

	dict := pdf.Dict{
		"Subtype": pdf.Name("FileAttachment"),
	}

	// Add common annotation fields
	if err := f.Common.fillDict(rm, dict); err != nil {
		return nil, err
	}

	// Add markup annotation fields
	if err := f.Markup.fillDict(rm, dict); err != nil {
		return nil, err
	}

	// Add file attachment-specific fields
	// FS (required)
	if f.FS != 0 {
		dict["FS"] = f.FS
	}

	// Name (optional) - only write if not the default value "PushPin"
	if f.Name != "" && f.Name != "PushPin" {
		dict["Name"] = f.Name
	}

	return dict, nil
}
