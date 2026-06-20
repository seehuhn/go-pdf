// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2023  Jochen Voss <voss@seehuhn.de>
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

package pdf

import (
	"errors"
	"fmt"
	"io"

	"seehuhn.de/go/membudget"
	"seehuhn.de/go/pdf/internal/limits"
)

// Getter represents a PDF file opened for reading.
//
// TODO(voss): find a better name for this
type Getter interface {
	GetMeta() *MetaInfo

	// Get reads an object from the file.
	//
	// The argument canObjStm specifies whether the object may be read from an
	// object stream.  Normally, this should be set to true.  If canObjStm is
	// false and the object is in an object stream, an error is returned.
	Get(ref Reference, canObjStm bool) (Native, error)
}

// Resolve resolves references to indirect objects.
//
// If obj is a [Reference], the function reads the corresponding object from
// the file and returns the result.  If obj is not a [Reference], it is
// returned unchanged.  The function recursively follows chains of references
// until it resolves to a non-reference object.
//
// If a reference loop is encountered, the function returns an error of type
// [MalformedFileError].
func Resolve(r Getter, obj Object) (Native, error) {
	return resolve(r, obj, true)
}

// maxFilterChainLength bounds the number of entries in a stream's
// /Filter array.  Legitimate PDFs rarely exceed two or three; the cap
// blocks malformed inputs that stack hundreds of decoders to amplify
// per-wrapper overhead.
const maxFilterChainLength = 8

// resolve follows indirect references until it reaches a non-reference object.
// See [resolvePath] for the cycle and depth bounds.  Pass canObjStm=false to
// forbid reading from an object stream (used while resolving stream lengths).
func resolve(r Getter, obj Object, canObjStm bool) (Native, error) {
	n, _, err := resolvePath(r, nil, obj, canObjStm)
	return n, err
}

// getInteger resolves an integer, reading from object streams if needed.
func getInteger(r Getter, obj Object) (Integer, error) {
	resolved, err := resolve(r, obj, true)
	if err != nil {
		return 0, err
	}
	return asInteger(resolved)
}

func getIntegerNoObjStm(r Getter, obj Object) (Integer, error) {
	resolved, err := resolve(r, obj, false)
	if err != nil {
		return 0, err
	}
	return asInteger(resolved)
}

// RawStreamReader returns a reader for the raw stream data after decryption
// but before any filter decoding.  This is used to copy a stream to another
// PDF file while preserving its original encoding.
//
// Each call creates a fresh reader, so streams can be read multiple times.
//
// Streams whose filter chain begins with an explicit /Crypt entry naming a
// CF other than /Identity (e.g. /StdCF, or a custom /CF entry) cannot be
// decrypted yet: the function returns an error rather than silently
// emitting ciphertext.  Encoding/decoding such streams is Phase 2 work.
func RawStreamReader(r Getter, x *Stream) (io.ReadCloser, error) {
	recipe, err := streamCryptRecipe(r, x)
	if err != nil {
		return nil, err
	}
	out := io.NopCloser(x.NewReader())
	switch recipe {
	case cryptNone, cryptIdentity:
		return out, nil
	case cryptDefault:
		v := GetVersion(r)
		budget := membudget.New(limits.StreamBudget(x.length))
		return x.crypt.Decode(v, out, budget)
	case cryptUnsupportedCF:
		return nil, errors.New(
			"RawStreamReader: stream uses non-Identity /Crypt filter; not yet supported")
	}
	panic("unreachable")
}

// cryptRecipe describes how a stream's on-disk bytes relate to its
// post-decryption form, controlling whether [RawStreamReader] needs to
// apply [filterCrypt] decryption and whether [Copier] can install the
// source bytes verbatim.
type cryptRecipe int

const (
	// cryptNone: the source document is unencrypted.  On-disk bytes
	// equal post-decryption bytes.
	cryptNone cryptRecipe = iota

	// cryptDefault: the document is encrypted and the stream uses the
	// default StmF crypt filter with per-object Algorithm 1 key
	// derivation.  Post-decryption bytes require [filterCrypt.Decode].
	cryptDefault

	// cryptIdentity: the document is encrypted but the stream's filter
	// chain declares /Crypt /Identity at position 0, overriding the
	// default StmF.  On-disk bytes are already plaintext.
	cryptIdentity

	// cryptUnsupportedCF: the stream's filter chain declares /Crypt
	// /StdCF or a named CF at position 0.  The bytes are encrypted
	// with that CF's recipe (no per-object Algorithm 1), which the
	// library does not yet implement.
	cryptUnsupportedCF
)

// streamCryptRecipe classifies how the encryption layer applies to x.
// It is shared between [RawStreamReader] and [Copier] so that both
// paths agree on which streams require decryption and which the
// library cannot yet handle.
func streamCryptRecipe(r Getter, x *Stream) (cryptRecipe, error) {
	if x.crypt == nil {
		return cryptNone, nil
	}
	// cheap probe: most streams have no /Crypt at filter position 0
	startsWithCrypt, err := filterChainStartsWithCrypt(r, x.Dict["Filter"])
	if err != nil {
		return 0, err
	}
	if !startsWithCrypt {
		return cryptDefault, nil
	}
	filters, err := GetFilters(r, nil, x.Dict)
	if err != nil {
		return 0, err
	}
	switch filters[0].(type) {
	case FilterCryptIdentity:
		return cryptIdentity, nil
	case CryptFilter:
		return cryptUnsupportedCF, nil
	}
	return cryptDefault, nil
}

// filterChainStartsWithCrypt reports whether the resolved /Filter entry
// names the Crypt filter at position 0.  This is a cheap probe used to
// short-circuit document-level (de)encryption without paying the cost
// of [GetFilters] (which resolves JBIG2Globals streams and rejects
// malformed filter chains).  The filter object and, for arrays, its
// first element are resolved through r so that an indirect /Filter or
// indirect first entry is handled correctly.
func filterChainStartsWithCrypt(r Getter, filter Object) (bool, error) {
	resolved, err := Resolve(r, filter)
	if err != nil {
		return false, err
	}
	switch f := resolved.(type) {
	case Name:
		return f == "Crypt", nil
	case Array:
		if len(f) == 0 {
			return false, nil
		}
		first, err := Resolve(r, f[0])
		if err != nil {
			return false, err
		}
		name, _ := first.(Name)
		return name == "Crypt", nil
	}
	return false, nil
}

// DecodeStream returns a reader for the decoded stream data. If numFilters is
// non-zero, only the first numFilters filters are decoded.
//
// Each call creates a fresh reader, so streams can be decoded multiple times.
//
// For encrypted PDFs, decryption is applied on-the-fly before any other
// filters. This does not count towards numFilters.
//
// Errors returned by Read on the resulting reader are classified per the
// package-level two-class error model: malformed-content errors are tagged
// [*MalformedFileError] (wrapped by the filter layer), and real failures
// from the underlying byte source propagate unchanged — even when a filter
// layer would otherwise have transformed them (e.g. flate emitting
// [io.ErrUnexpectedEOF]), because a sticky source-error tracker promotes
// the original error at the top of the stack.
func DecodeStream(r Getter, path *CycleCheck, x *Stream) (io.ReadCloser, error) {
	filters, err := GetFilters(r, path, x.Dict)
	if err != nil {
		return nil, err
	}

	v := GetVersion(r)

	src := &sourceErrChecker{r: x.NewReader()}
	var out io.ReadCloser = io.NopCloser(src)

	// per-decode working-memory budget, sized to the raw stream length
	budget := membudget.New(limits.StreamBudget(x.length))

	// Skip the document-level decryption when the stream's filter chain
	// declares its own per-stream Crypt filter at position 0.  Per PDF
	// spec §7.4.10, an explicit /Crypt entry overrides the document's
	// default StmF and skips the per-object Algorithm 1 key derivation;
	// the [CryptFilter] variant at position 0 is responsible for
	// decryption (a pass-through for [FilterCryptIdentity]).
	applyCrypt := x.crypt != nil
	if applyCrypt && len(filters) > 0 {
		if _, ok := filters[0].(CryptFilter); ok {
			applyCrypt = false
		}
	}
	if applyCrypt {
		out, err = x.crypt.Decode(v, out, budget)
		if err != nil {
			return nil, src.promote(err)
		}
	}

	for _, fi := range filters {
		out, err = fi.Decode(v, out, budget)
		if err != nil {
			return nil, src.promote(err)
		}
	}
	return &sourceAwareReader{inner: out, src: src}, nil
}

// sourceErrChecker wraps the raw byte source underlying a decoded PDF
// stream. It is sticky: once a non-EOF error is observed, it is recorded
// and returned by Err() for the rest of the reader's lifetime. The filter
// chain above reads through this wrapper transparently; [sourceAwareReader]
// uses the recorded error to recover source failures that filter layers
// may have replaced with their own error (e.g. flate returning
// [io.ErrUnexpectedEOF] when the source truncates unexpectedly).
type sourceErrChecker struct {
	r      io.Reader
	srcErr error
}

func (s *sourceErrChecker) Read(p []byte) (int, error) {
	n, err := s.r.Read(p)
	if err != nil && !errors.Is(err, io.EOF) && s.srcErr == nil {
		s.srcErr = err
	}
	return n, err
}

// Err returns the sticky non-EOF source error, or nil.
func (s *sourceErrChecker) Err() error { return s.srcErr }

// promote returns the sticky source error if one has been recorded,
// otherwise the supplied err. It is used while the filter chain is
// being constructed, so that a source-level IO failure observed during
// a filter's header read is surfaced to the caller instead of the
// (content-classified) error the filter returns.
func (s *sourceErrChecker) promote(err error) error {
	if s.srcErr != nil {
		return s.srcErr
	}
	return err
}

// sourceAwareReader is the outermost wrapper returned by [DecodeStream].
// When the filter chain produces a non-nil error, it checks whether the
// underlying byte source has recorded an error of its own; if so, the
// source error wins, so real IO failures surface to the caller even when
// an intermediate filter layer has substituted its own content error.
type sourceAwareReader struct {
	inner io.ReadCloser
	src   *sourceErrChecker
}

func (s *sourceAwareReader) Read(p []byte) (int, error) {
	n, err := s.inner.Read(p)
	if err != nil && s.src.srcErr != nil {
		err = s.src.srcErr
	}
	return n, err
}

func (s *sourceAwareReader) Close() error { return s.inner.Close() }

// GetFilters extracts the information contained in the /Filter and
// /DecodeParms entries of a stream dictionary.
func GetFilters(r Getter, path *CycleCheck, dict Dict) ([]Filter, error) {
	decodeParams, err := resolve(r, dict["DecodeParms"], false)
	if err != nil {
		return nil, err
	}
	filter, err := resolve(r, dict["Filter"], false)
	if err != nil {
		return nil, err
	}

	var res []Filter
	switch f := filter.(type) {
	case nil:
		// pass
	case Name:
		var pDict Dict
		if decodeParams != nil {
			var ok bool
			pDict, ok = decodeParams.(Dict)
			if !ok {
				return nil, fmt.Errorf("wrong type, expected Dict but got %T", decodeParams)
			}
		}
		filter, err := MakeFilter(f, pDict)
		if err != nil {
			return nil, err
		}
		res = append(res, filter)
	case Array:
		if len(f) > maxFilterChainLength {
			return nil, &MalformedFileError{
				Err: fmt.Errorf("filter chain length %d exceeds limit", len(f)),
			}
		}
		pa, ok := decodeParams.(Array)
		if !ok && decodeParams != nil {
			return nil, errors.New("invalid /DecodeParms field")
		}
		for i, fi := range f {
			fi, err := resolve(r, fi, false)
			if err != nil {
				return nil, err
			}
			name, ok := fi.(Name)
			if !ok {
				return nil, fmt.Errorf("wrong type, expected Name but got %T", fi)
			}
			var pDict Dict
			if len(pa) > i {
				pai, err := resolve(r, pa[i], false)
				if err != nil {
					return nil, err
				}
				if pai != nil {
					var ok bool
					pDict, ok = pai.(Dict)
					if !ok {
						return nil, fmt.Errorf("wrong type, expected Dict but got %T", pai)
					}
				}
			}
			filter, err := MakeFilter(name, pDict)
			if err != nil {
				return nil, err
			}
			res = append(res, filter)
		}
	default:
		return nil, Error("invalid /Filter field")
	}

	// Per PDF spec §7.4.10, a Crypt filter must be the first entry in
	// the /Filter array.  Reject malformed input where it appears at
	// any other position: there is no recoverable interpretation of an
	// out-of-position Crypt filter.
	for i, f := range res {
		if _, ok := f.(CryptFilter); ok && i != 0 {
			return nil, &MalformedFileError{
				Err: errors.New("Crypt filter must be at position 0 in /Filter"),
			}
		}
	}

	// resolve JBIG2Globals for any JBIG2Decode filters
	for _, f := range res {
		if jf, ok := f.(*FilterJBIG2); ok {
			if err := resolveJBIG2Globals(r, path, jf); err != nil {
				return nil, err
			}
		}
	}

	return res, nil
}

// resolveJBIG2Globals reads the JBIG2Globals stream from the filter's
// DecodeParms dictionary and stores the bytes in the filter.
//
// f.GlobalsRef is preserved after resolution so that callers writing
// the stream back out (without going through [Copier]) can decide
// whether to keep, remap, or drop the reference.
func resolveJBIG2Globals(r Getter, path *CycleCheck, f *FilterJBIG2) error {
	if f.GlobalsRef == nil {
		return nil
	}

	// detect cycles in chains of /JBIG2Globals references
	if ref, isRef := f.GlobalsRef.(Reference); isRef {
		if path.Seen(ref) {
			return &MalformedFileError{
				Err: ErrCycle,
				Loc: []string{"JBIG2Globals " + ref.String()},
			}
		}
		path = &CycleCheck{Ref: ref, Parent: path}
	}

	// resolve the reference to get the stream
	obj, err := resolve(r, f.GlobalsRef, false)
	if err != nil {
		return err
	}
	if obj == nil {
		return nil
	}

	// the globals object should be a stream; read its contents
	stream, ok := obj.(*Stream)
	if !ok {
		return nil
	}
	data, err := ReadAll(r, path, stream, limits.MaxJBIG2GlobalsBytes)
	if err != nil {
		return fmt.Errorf("reading JBIG2Globals: %w", err)
	}
	f.Globals = data
	return nil
}

// IsTagged returns true, if the PDF file is "tagged".
func IsTagged(pdf *Writer) bool {
	// TODO(voss): what can we do if catalog.MarkInfo is an indirect object?
	catalog := pdf.GetMeta().Catalog
	markInfo, _ := catalog.MarkInfo.(Dict)
	if markInfo == nil {
		return false
	}
	marked, _ := markInfo["Marked"].(Boolean)
	return bool(marked)
}

// GetVersion returns the PDF version used in a PDF file.
func GetVersion(pdf interface{ GetMeta() *MetaInfo }) Version {
	return pdf.GetMeta().Version
}
