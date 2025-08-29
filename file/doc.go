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

// Package file implements PDF file specification dictionaries and related objects.
//
// File specifications provide a way to reference files that are external to a PDF
// document or embedded within it. This package supports both string and dictionary
// forms of file specifications as defined in PDF 2.0 sections 7.11.3 and 7.11.4.
//
// The main types provided are:
//   - [Specification]: represents a file specification dictionary
//   - [EncryptedPayload]: represents an encrypted payload dictionary for encrypted files
//   - [RelatedFile]: represents entries in related files arrays
//   - [Relationship]: represents the relationship between components and associated files
//
// File specifications can reference both external files and embedded files within
// the PDF document. They support various platform-specific file naming conventions
// and provide metadata about the referenced files.
package file
