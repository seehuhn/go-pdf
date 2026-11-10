// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

// Package oc implements optional content (PDF 1.5, section 8.11),
// the mechanism behind layers in PDF documents. Optional content
// allows parts of a document to be selectively viewed or hidden.
//
// The main types are:
//
//   - [Group] is a named toggle that controls visibility of associated content.
//   - [Membership] computes visibility from a boolean combination of groups.
//   - [Configuration] defines initial group states and UI presentation.
//   - [Properties] is the top-level container stored in the document catalog.
//
// A [Group] or [Membership] (both implement [Conditional]) is associated
// with content via marked content sequences (BDC/EMC) in content streams.
//
// Group visibility is held in a [ViewState]: the states a configuration
// prescribes, obtained with [Configuration.DefaultState], together with the
// manual changes the user or a set-OCG-state action has made.  Rendering
// consults a [GroupStates] snapshot derived from the view state with
// [ViewState.Effective], which folds in the configuration's usage
// recommendations for the current view.  Groups whose intent does not match
// the configuration are excluded from the state, so they have no effect on
// visibility.
package oc
