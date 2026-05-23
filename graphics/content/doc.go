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

// Package content implements PDF content streams and resource dictionaries.
//
// A content stream is a sequence of PDF operators describing what to paint
// on a page (or inside a Form XObject, tiling pattern, Type 3 glyph
// procedure, etc.).  Each content stream depends on a resource dictionary
// that supplies the named resources (fonts, images, patterns, ExtGStates,
// …) the operators reference.
//
// # Validation responsibility
//
// All construction-time validation lives in the
// [seehuhn.de/go/pdf/graphics/content/builder.Builder] type:
//
//   - operators unknown or unavailable in the chosen PDF version are
//     rejected at emit time;
//   - deprecated operators (e.g. F) are rejected — callers must use the
//     modern typed helper (Fill);
//   - structural rules (q/Q stack depth, q/Q-in-text-object for pre-2.0)
//     are enforced at the offending operator;
//   - operators that fail [State.ApplyOperator] (improper nesting, wrong
//     graphics-object context, required state not set) are rejected.
//
// The rest of the package trusts that the operator stream it sees is
// well-formed:
//
//   - [Operator.Format], [Operators.RawBytes] and [Operators.Embed] are
//     thin serialisers; they do not validate.
//   - [NewScanner] yields the raw operator stream the scanner saw,
//     without any rewrites, drops, or synthesised closers.
//   - [Iter] reports IO errors via [Iter.Err]; it does not synthesise
//     closing operators for unbalanced contexts.
//
// # Reading malformed content
//
// The library aims to be permissive when reading.  Consumers advance
// their [State] by calling [State.ApplyStateChanges] directly, which
// skips the context and required-state checks that
// [State.ApplyOperator] performs for the builder.  Consumers that want
// the stream to look well-formed after the fact can drain
// [State.ClosingOperators] at end-of-stream to emit synthetic closers
// for any contexts left open.  [seehuhn.de/go/pdf/reader.Reader] does
// exactly this.
//
// The one Builder bypass is the [OpRawContent] pseudo-operator and any
// Builder method that emits it directly: bytes injected this way are
// written verbatim and not validated.
package content
