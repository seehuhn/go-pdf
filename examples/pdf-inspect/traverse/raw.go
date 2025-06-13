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

package traverse

import (
	"io"
	"os"
)

type rawStreamCtx struct {
	r io.Reader
}

func (ctx *rawStreamCtx) Next(key string) (Context, error) {
	return nil, &KeyError{Key: key, Ctx: "@raw stream"}
}

func (ctx *rawStreamCtx) Show() error {
	_, err := io.Copy(os.Stdout, ctx.r)
	return err
}

func (ctx *rawStreamCtx) Keys() ([]string, error) {
	return nil, nil
}
