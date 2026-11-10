// Copyright 2021 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

// Package pdf provides support for reading and writing PDF files.
//
// This package treats PDF files as containers containing a sequence of objects
// (typically Dictionaries and Streams).  Object are written sequentially, but
// can be read in any order.  These objects represent pages of text, fonts,
// images and so on.  Subpackages implement support to produce PDF files
// representing pages of text and images.
//
// A [Reader] can be used to read objects from an existing PDF file:
//
//	r, err := pdf.Open("in.pdf")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer r.Close()
//	... use r.Catalog to locate objects in the file ...
//
// A [Writer] can be used to write objects to a new PDF file:
//
//	w, err := pdf.Create("out.pdf")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	... add objects to the document using w.Write() and w.OpenStream() ...
//
//	w.Catalog.Pages = ... // set the page tree
//
//	err = out.Close()
//	if err != nil {
//	    log.Fatal(err)
//	}
//
// [Update], [NewUpdater] and [NewUpdaterTo] append an incremental update to
// an existing file.
//
// The following classes represent the native PDF object types: [Array],
// [Boolean], [Dict], [Integer], [Name], [Real], [Reference], [*Stream], [String].
// All of these implement the [Object] interface.
package pdf
