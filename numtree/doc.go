// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

// Package numtree implements PDF number trees.
//
// Number trees serve a similar purpose to dictionaries, associating keys and
// values, but using integer keys instead of name objects. The keys are stored
// in sorted numerical order, and the data structure can represent an
// arbitrarily large collection of key-value pairs with efficient lookup
// without requiring the entire structure to be read from the PDF file.
//
// This package provides two implementations:
//   - InMemory: loads the entire tree into memory for fast access
//   - FromFile: reads nodes on demand for memory-efficient access to large trees
//
// The Write function creates balanced number trees using a streaming B-tree
// algorithm that processes entries incrementally with O(log n) memory usage.
package numtree
