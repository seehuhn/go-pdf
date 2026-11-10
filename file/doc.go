// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

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
