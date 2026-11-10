// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

// Package formhooks lets the acroform and annotation packages call into each
// other without an import cycle.
//
// The function variables are set by the annotation package when it is
// initialised and called by the acroform package. This package must not import
// either of them, so the hooks pass values as untyped any.
package formhooks

import "seehuhn.de/go/pdf"

// EncodeWidgetEntries returns the widget annotation's own dictionary entries
// (everything except the form-field linkage: /Parent and the folded-in field
// entries). The acroform package calls it while encoding a form to assemble the
// merged or separate field/widget dictionary; the annotation package sets it
// when it is initialised. The widget is passed as an untyped value (the
// *annotation.Widget).
var EncodeWidgetEntries func(rm *pdf.ResourceManager, widget any) (pdf.Dict, error)
