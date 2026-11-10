// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package traverse

import (
	"io"
	"os"
)

type rawStreamCtx struct {
	r io.Reader
}

func (ctx *rawStreamCtx) Next() []Step {
	return nil
}

func (ctx *rawStreamCtx) Show() error {
	_, err := io.Copy(os.Stdout, ctx.r)
	return err
}
