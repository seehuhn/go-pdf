// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

// Package outline reads and writes PDF document outlines.
//
// A document outline is a tree-structured hierarchy of outline items
// (sometimes called bookmarks) that PDF viewers typically display as a
// navigable table of contents. Each item can have a title, destination or
// action, and optional styling (color, bold, italic).  A typical use would be
// to represent the chapter and section structure of a document.
package outline
