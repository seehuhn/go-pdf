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

// Package action implements PDF actions as defined in section 12.6 of the
// PDF 2.0 specification.
//
// Actions define operations that can be triggered by user interaction,
// document opening, or other events. This package provides support for all
// 19 standard action types including navigation (GoTo, GoToR, URI),
// form operations (SubmitForm, ResetForm, ImportData), multimedia control
// (Sound, Movie, Rendition), and scripting (JavaScript).
//
// # Basic Usage
//
// Create an action and encode it:
//
//	action := &action.GoTo{
//		Dest: &destination.XYZ{
//			Page: pageRef,
//			Left: 100,
//			Top:  200,
//			Zoom: 1.5,
//		},
//	}
//	dict, err := action.Encode(rm)
//
// Decode an action from a PDF object:
//
//	action, err := action.Decode(x, obj)
//	switch a := action.(type) {
//	case *action.GoTo:
//		// handle go-to action
//	case *action.URI:
//		// handle URI action
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
// Navigation actions: GoTo, GoToR, GoToE, GoToDp, Thread
//
// Web and external: URI, Launch
//
// Predefined: Named (NextPage, PrevPage, FirstPage, LastPage)
//
// Form actions: SubmitForm, ResetForm, ImportData
//
// Multimedia: Sound (deprecated), Movie (deprecated), Rendition
//
// Annotation: Hide
//
// Scripting: JavaScript
//
// Optional content: SetOCGState
//
// 3D and rich media: GoTo3DView, RichMediaExecute, Trans
package action
