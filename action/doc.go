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

// Package action implements PDF actions.
//
// Actions define operations that can be triggered by user interaction,
// document opening, or other events. This package provides support for all 20
// standard action types including navigation (GoTo, GoToR, URI), form
// operations (SubmitForm, ResetForm, ImportData), multimedia control (Sound,
// Movie, Rendition), and scripting (JavaScript).
//
// # Basic Usage
//
// Create an action and encode it:
//
//	action := &action.GoTo{
//	    Dest: &destination.XYZ{
//	        Page: pageRef,
//	        Left: 100,
//	        Top:  200,
//	        Zoom: 1.5,
//	    },
//	}
//	dict, err := action.Encode(rm)
//
// Decode an action from a PDF object. Always invoke the decoder via
// [pdf.ExtractorGet] so that indirect references are resolved and cycle
// detection is preserved:
//
//	a, err := pdf.ExtractorGet(x, nil, obj, action.Decode)
//	switch a := a.(type) {
//	case *action.GoTo:
//	    // handle go-to action
//	case *action.URI:
//	    // handle URI action
//	}
//
// # Action Chaining
//
// Actions can be chained using the Next field:
//
//	action1 := &action.URI{URI: "https://example.com"}
//	action2 := &action.Named{N: "NextPage"}
//	action1.Next = action.ActionList{action2}
//
// The Next field can contain multiple actions that execute in parallel:
//
//	action.Next = action.ActionList{action2, action3, action4}
//
// # Action Types
//
// Navigation actions:
//
//   - [GoTo] navigates to a destination in the current document.
//   - [GoToR] navigates to a destination in another PDF file.
//   - [GoToE] navigates to a destination in an embedded file.
//   - [GoToDp] navigates to a document part in the current document (PDF 2.0).
//   - [Thread] begins reading an article thread.
//
// Web and external actions:
//
//   - [URI] resolves a uniform resource identifier.
//   - [Launch] launches an application or opens or prints a document.
//
// Predefined actions:
//
//   - [Named] executes a predefined action (NextPage, PrevPage, FirstPage, LastPage).
//
// Form actions:
//
//   - [SubmitForm] sends form data to a URL.
//   - [ResetForm] resets form fields to their default values.
//   - [ImportData] imports form field values from a file.
//
// Multimedia actions:
//
//   - [Rendition] controls multimedia playback.
//   - [Sound] plays a sound (deprecated in PDF 2.0).
//   - [Movie] plays a movie (deprecated in PDF 2.0).
//
// Display actions:
//
//   - [Trans] updates the display using a transition dictionary.
//   - [Hide] shows or hides annotations.
//
// Scripting actions:
//
//   - [JavaScript] executes ECMAScript.
//
// Optional content actions:
//
//   - [SetOCGState] sets the state of optional content groups.
//
// 3D and rich media actions:
//
//   - [GoTo3DView] sets the current view of a 3D annotation.
//   - [RichMediaExecute] sends a command to a rich media annotation's handler (PDF 2.0).
package action
