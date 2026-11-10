// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package traverse

import (
	"seehuhn.de/go/pdf"

	"seehuhn.de/go/pdf/cmd/pdf-inspect/meta"
)

type metaCtx struct {
	r pdf.Getter
}

func (c *metaCtx) Next() []Step {
	return nil
}

func (c *metaCtx) Show() error {
	return meta.ShowMetadata(c.r)
}
