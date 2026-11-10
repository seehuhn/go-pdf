// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package annotation

import (
	"errors"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/formhooks"
)

// register the widget plumbing used by the acroform package, so that it need
// not be part of the public API
func init() {
	formhooks.EncodeWidgetEntries = func(rm *pdf.ResourceManager, widget any) (pdf.Dict, error) {
		w, ok := widget.(*Widget)
		if !ok {
			return nil, errors.New("not a widget annotation")
		}
		return w.encodeOwnEntries(rm)
	}
}
