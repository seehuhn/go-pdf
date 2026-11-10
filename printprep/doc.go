// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

// Package printprep rewrites a PDF document into a print-safe simplification.
//
// The output is a fresh, unencrypted PDF that contains only the visible,
// printable content of the source, using font constructions that render
// reliably across a broad range of PDF renderers and print pipelines.  It is
// intended to be handed to a host print system for rasterization.
//
// The transformation:
//
//   - normalizes fonts so that renderers do not have to interpret unusual
//     font constructions (see the font policy in [Write]);
//   - removes the content of optional-content groups that are switched off,
//     and drops the optional-content machinery entirely;
//   - flattens printable annotations into page content, synthesizing an
//     appearance where the file has none, and drops those the print
//     visibility rules suppress, as well as replies, which no viewer draws
//     on their own;
//   - strips tagged-PDF structure, metadata, navigation, and other data that
//     does not affect the printed marks;
//   - writes a fresh, unencrypted document.
//
// The package is platform-independent; it knows nothing about any particular
// operating system or renderer.
package printprep
