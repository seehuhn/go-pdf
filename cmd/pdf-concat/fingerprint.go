// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2026  Jochen Voss <voss@seehuhn.de>
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

package main

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"hash"
	"io"
	"math"
	"slices"

	"seehuhn.de/go/pdf"
)

// fingerprintBudget is the number of bytes which may be hashed, per byte of
// the input file, when fingerprinting the default resources of one input.
// Each fingerprint hashes all objects reachable from its resource, so
// resources which share large objects hash them repeatedly; the budget keeps
// the total cost proportional to the file size.
const fingerprintBudget = 32

// resourceSum is a fingerprint of a resource of /DR and of all objects
// reachable from it.  Resources with the same fingerprint are equivalent,
// even when they come from different files.
type resourceSum [sha256.Size]byte

// errNoFingerprint reports that a resource has no fingerprint, because part
// of it is malformed or the budget is used up.
var errNoFingerprint = errors.New("object cannot be fingerprinted")

// resourceKey identifies a resource of /DR.
type resourceKey struct {
	category, name pdf.Name
}

// fingerprinter computes fingerprints of the default resources of one input.
//
// The objects reachable from a resource are serialised in depth-first order,
// with dictionary keys sorted.  A reference seen before is serialised as its
// position in the order of first visits, so that the serialisation also
// records which objects are shared, and cycles end.  Stream data enters in
// decoded form, and the stream dictionary entries which only describe the
// encoding are left out.
//
// A /DA string is serialised by its meaning, not its text: its operators are
// serialised one by one, and an operand naming a resource is replaced by the
// serialisation of the resource it names in the input's /DR.  Resources are
// renamed when inputs are merged, and /DA strings rewritten to match, so the
// fingerprint does not depend on the names in use.
type fingerprinter struct {
	r      pdf.Getter
	dr     map[pdf.Name]pdf.Dict // the input's /DR, by category
	budget int64                 // the number of bytes which may still be hashed

	h    hash.Hash
	seen map[any]int // first visits of references and resourceKeys
}

func newFingerprinter(r pdf.Getter, dr map[pdf.Name]pdf.Dict, budget int64) *fingerprinter {
	return &fingerprinter{
		r:      r,
		dr:     dr,
		budget: budget,
		h:      sha256.New(),
		seen:   map[any]int{},
	}
}

// sum computes the fingerprint of the given resource of /DR.
//
// The returned bool is false where the resource has no fingerprint, because
// some of it is malformed or the budget is used up.  Such a resource is not
// equivalent to any other.
func (f *fingerprinter) sum(category, name pdf.Name) (resourceSum, bool, error) {
	f.h.Reset()
	clear(f.seen)
	err := f.resource(resourceKey{category, name})
	if errors.Is(err, errNoFingerprint) {
		return resourceSum{}, false, nil
	} else if err != nil {
		return resourceSum{}, false, err
	}
	var sum resourceSum
	f.h.Sum(sum[:0])
	return sum, true, nil
}

// resource serialises a resource of /DR.  A missing resource is serialised
// by its name, which the merged /DR keeps reserved where it can.
func (f *fingerprinter) resource(key resourceKey) error {
	if idx, ok := f.seen[key]; ok {
		f.tag('@')
		f.int(int64(idx))
		return nil
	}
	f.seen[key] = len(f.seen)
	val := f.dr[key.category][key.name]
	if val == nil {
		f.tag('m')
		f.bytes([]byte(key.name))
		return nil
	}
	f.tag('N')
	return f.walk(val)
}

// encodingKeys are the stream dictionary entries which describe how the
// stream data is encoded in the file.
var encodingKeys = map[pdf.Name]bool{
	"Length":      true,
	"Filter":      true,
	"DecodeParms": true,
	"DL":          true,
}

func (f *fingerprinter) walk(obj pdf.Object) error {
	if f.budget < 0 {
		return errNoFingerprint
	}
	switch x := obj.(type) {
	case nil:
		f.tag('n')
	case pdf.Reference:
		if idx, ok := f.seen[x]; ok {
			f.tag('@')
			f.int(int64(idx))
			return nil
		}
		f.seen[x] = len(f.seen)
		f.tag('R')
		native, err := f.r.Get(x, true)
		if pdf.IsReadError(err) {
			return err
		} else if err != nil {
			return errNoFingerprint
		}
		return f.walk(native)
	case pdf.Boolean:
		f.tag('b')
		if x {
			f.tag(1)
		} else {
			f.tag(0)
		}
	case pdf.Integer:
		f.tag('i')
		f.int(int64(x))
	case pdf.Real:
		f.tag('r')
		f.int(int64(math.Float64bits(float64(x))))
	case pdf.Name:
		f.tag('/')
		f.bytes([]byte(x))
	case pdf.String:
		f.tag('(')
		f.bytes(x)
	case pdf.Array:
		f.tag('[')
		f.int(int64(len(x)))
		for _, el := range x {
			if err := f.walk(el); err != nil {
				return err
			}
		}
	case pdf.Dict:
		f.tag('<')
		return f.dict(x, false)
	case *pdf.Stream:
		f.tag('s')
		if err := f.dict(x.Dict, true); err != nil {
			return err
		}
		return f.streamData(x)
	default:
		return errNoFingerprint
	}
	return nil
}

// dict serialises the entries of d, leaving out those with a nil value.
// For a stream dictionary, the entries in encodingKeys are left out too.
// Otherwise, a /DA string is serialised by its meaning, matching the
// dictionaries in which [daRenamer] rewrites it.
func (f *fingerprinter) dict(d pdf.Dict, isStream bool) error {
	var keys []pdf.Name
	for key, val := range d {
		if val != nil && !(isStream && encodingKeys[key]) {
			keys = append(keys, key)
		}
	}
	slices.Sort(keys)
	f.int(int64(len(keys)))
	for _, key := range keys {
		f.bytes([]byte(key))
		if key == "DA" && !isStream {
			resolved, err := pdf.Resolve(f.r, d[key])
			if pdf.IsReadError(err) {
				return err
			}
			if da, ok := resolved.(pdf.String); ok {
				if err := f.da(da); err != nil {
					return err
				}
				continue
			}
		}
		if err := f.walk(d[key]); err != nil {
			return err
		}
	}
	return nil
}

// da serialises a default appearance string by its meaning.
func (f *fingerprinter) da(da pdf.String) error {
	f.spend(len(da))
	ops := daOperators(da)
	f.tag('D')
	f.int(int64(len(ops)))
	for _, op := range ops {
		f.bytes([]byte(op.Name))
		f.int(int64(len(op.Args)))
		category, idx, isRes := daResourceArg(op)
		for i, arg := range op.Args {
			var err error
			if isRes && i == idx {
				err = f.resource(resourceKey{category, arg.(pdf.Name)})
			} else {
				err = f.walk(arg)
			}
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// streamData adds the hash of the decoded stream data.
func (f *fingerprinter) streamData(x *pdf.Stream) error {
	rc, err := pdf.DecodeStream(f.r, nil, x)
	if pdf.IsReadError(err) {
		return err
	} else if err != nil {
		return errNoFingerprint
	}
	defer rc.Close()
	h := sha256.New()
	n, err := io.Copy(h, io.LimitReader(rc, max(f.budget+1, 0)))
	f.budget -= n
	if pdf.IsReadError(err) {
		return err
	} else if err != nil || f.budget < 0 {
		return errNoFingerprint
	}
	f.write(h.Sum(nil))
	return nil
}

func (f *fingerprinter) tag(b byte) {
	f.write([]byte{b})
}

func (f *fingerprinter) int(v int64) {
	f.write(binary.BigEndian.AppendUint64(nil, uint64(v)))
}

// bytes writes b, prefixed by its length so that the serialisation can be
// split into its parts unambiguously.
func (f *fingerprinter) bytes(b []byte) {
	f.int(int64(len(b)))
	f.write(b)
}

func (f *fingerprinter) write(b []byte) {
	f.spend(len(b))
	f.h.Write(b)
}

// spend charges n bytes of work against the budget.
func (f *fingerprinter) spend(n int) {
	f.budget -= int64(n)
}
