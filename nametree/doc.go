// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

// Package nametree implements PDF name trees.
//
// Name trees serve a similar purpose to dictionaries, associating keys and
// values, but using string keys that are ordered lexicographically. The data
// structure can represent an arbitrarily large collection of key-value pairs
// with efficient lookup without requiring the entire structure to be read
// from the PDF file.
//
// This package provides two implementations:
//   - InMemory: loads the entire tree into memory for fast access
//   - FromFile: reads nodes on demand for memory-efficient access to large trees
//
// The Write function creates balanced name trees using a streaming B-tree
// algorithm that processes entries incrementally with O(log n) memory usage.
package nametree
