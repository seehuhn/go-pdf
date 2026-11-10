// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

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
