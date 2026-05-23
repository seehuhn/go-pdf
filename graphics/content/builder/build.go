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

package builder

import "seehuhn.de/go/pdf/graphics/content"

// Build resets the builder, runs buildFunc to populate the stream,
// and returns the resulting [*content.Operators] segment.
// On error, it returns nil and the error is stored in b.Err.
func (b *Builder) Build(buildFunc func(b *Builder) error) *content.Operators {
	if b.Err != nil {
		return nil
	}
	b.Reset()
	if err := buildFunc(b); err != nil {
		b.Err = err
		return nil
	}
	if err := b.Close(); err != nil {
		b.Err = err
		return nil
	}
	res := b.Stream
	b.Stream = nil
	return &content.Operators{Ops: res}
}

// MustBuild is like Build but panics on error.
func (b *Builder) MustBuild(buildFunc func(b *Builder) error) *content.Operators {
	stream := b.Build(buildFunc)
	if b.Err != nil {
		panic(b.Err)
	}
	return stream
}

// Must converts the (value, error) return of [Builder.Harvest] or
// [Builder.Build] into a single value, panicking if the error is
// non-nil.  Typical usage:
//
//	form.Content = builder.Must(b.Harvest())
//
// Builder errors are programmer errors (invalid operator combinations,
// version mismatch, …) rather than runtime conditions, so panicking is
// appropriate at call sites that have no recovery path.
func Must(ops *content.Operators, err error) *content.Operators {
	if err != nil {
		panic(err)
	}
	return ops
}
