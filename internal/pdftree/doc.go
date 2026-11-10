// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

// Package pdftree implements the shared B-tree machinery behind PDF name trees
// and number trees (PDF 32000-2 sections 7.9.6 and 7.9.7).
//
// The two kinds of tree differ only in their key type and in how keys are
// stored in the file.  That variation is captured by the unexported codec
// interface, with [NameCodec] and [NumCodec] as its two instantiations.  The
// seehuhn.de/go/pdf/nametree and seehuhn.de/go/pdf/numtree packages are thin
// facades over this core.
package pdftree
