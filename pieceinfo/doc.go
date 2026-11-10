// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

// Package pieceinfo implements support for page-piece dictionaries.
//
// Page-piece dictionaries allow PDF processors to store private,
// application-specific data associated with documents, pages, or form
// XObjects. Each entry in a page-piece dictionary has a key which identifies
// the application the data is for, and the corresponding value has a
// LastModified date and contains private data, specific to the application.
//
// This package provides a generic framework for reading and writing page-piece
// dictionaries, with support for registering custom handlers for specific PDF
// processor types.
package pieceinfo
