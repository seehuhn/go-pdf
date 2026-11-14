// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package extract

import (
	"errors"
	"math"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/postscript/cid"
)

// getSimpleWidths populates ww with glyph widths from a font dictionary.
// It sets default widths for all glyphs, then updates specific widths from
// the Widths array in fontDict.
//
// Returns true if widths were successfully read.
func getSimpleWidths(ww []float64, c pdf.Cursor, fontDict pdf.Dict, defaultWidth float64) bool {
	for i := range ww {
		ww[i] = defaultWidth
	}

	firstChar, _ := c.Integer(fontDict["FirstChar"])
	widths, _ := c.Array(fontDict["Widths"])
	if widths == nil || len(widths) > 256 || firstChar < 0 || firstChar >= 256 {
		return false
	}

	for i, w := range widths {
		w, err := c.Number(w)
		if err != nil {
			continue
		}
		if code := firstChar + pdf.Integer(i); code < 256 {
			ww[code] = w
		}
	}
	return true
}

// decodeCompositeWidths reads glyph width information from a composite
// font's W array and returns a map from CID to width value.
//
// The W entry is optional.  If it is not an array, or if it assigns no
// widths, the result is nil.  If the array is malformed, the groups before the
// first malformed one are kept.  If groups overlap, the first group which
// covers a CID gives its width.  Only read errors are returned.
func decodeCompositeWidths(c pdf.Cursor, obj pdf.Object) (map[cid.CID]float64, error) {
	w, err := c.Array(obj)
	if pdf.IsReadError(err) {
		return nil, err
	} else if w == nil {
		return nil, nil
	}

	type group struct {
		cidRange
		width  float64   // for the range format
		widths []float64 // for the list format
	}
	var groups []group
	lists := make(listCache[float64])
	number1 := func(a pdf.Array) (float64, error) { return number(c, a[0]) }
groupLoop:
	for len(w) > 1 {
		c0, err := c.Integer(w[0])
		if err != nil {
			if pdf.IsReadError(err) {
				return nil, err
			}
			break
		}
		if c0 < 0 || c0 > maxCID {
			break
		}
		var g group
		g.first = int(c0)

		ww, isList, other, err := lists.get(c, w[1], 1, number1)
		switch {
		case pdf.IsReadError(err):
			return nil, err
		case isList:
			if err != nil || int(c0)+len(ww) > maxCID+1 {
				break groupLoop
			}
			g.last = int(c0) + len(ww) - 1
			g.widths = ww
			w = w[2:]
		default:
			c1, err := c.Integer(other)
			if err != nil || len(w) < 3 || c1 < c0 || c1 > maxCID {
				break groupLoop
			}
			g.width, err = number(c, w[2])
			if pdf.IsReadError(err) {
				return nil, err
			} else if err != nil {
				break groupLoop
			}
			g.last = int(c1)
			w = w[3:]
		}
		groups = append(groups, g)
	}

	// an empty W array is never written, so it reads as nil
	var res map[cid.CID]float64
	ranges := make([]cidRange, len(groups))
	for k, g := range groups {
		ranges[k] = g.cidRange
	}
	earliest(ranges, func(k, c int) {
		if res == nil {
			res = make(map[cid.CID]float64)
		}
		g := &groups[k]
		if g.widths != nil {
			res[cid.CID(c)] = g.widths[c-g.first]
		} else {
			res[cid.CID(c)] = g.width
		}
	})
	return res, nil
}

// maxCID is the largest valid CID value.
const maxCID = 65535

// cidRange is a range first, ..., last of CID values.  The range is empty if
// last < first.
type cidRange struct {
	first, last int
}

// earliest calls assign(k, c) once for every CID c covered by one of the
// ranges, where k is the index of the first range which covers c.  The work is
// proportional to len(ranges) plus the number of CIDs covered, up to a
// logarithmic factor.
func earliest(ranges []cidRange, assign func(k, c int)) {
	// next[c] links a covered CID c towards the next uncovered CID.  CIDs
	// not in the map are uncovered.
	next := make(map[int]int)
	find := func(c int) int {
		root := c
		for {
			p, ok := next[root]
			if !ok {
				break
			}
			root = p
		}
		for c != root {
			c, next[c] = next[c], root
		}
		return root
	}
	for k, r := range ranges {
		for c := find(r.first); c <= r.last; c = find(c + 1) {
			assign(k, c)
			next[c] = c + 1
		}
	}
}

// listCache holds the resolved second elements of the groups in W and W2,
// keyed by every reference followed to reach them.  While a chain is being
// followed, its references map to nil.  Each indirect object is
// thus read, and each array decoded, at most once, however many groups and
// chains of references lead to it.
type listCache[T any] map[pdf.Reference]*cachedList[T]

type cachedList[T any] struct {
	values []T
	isList bool
	other  pdf.Native
	err    error
}

// get decodes obj as an array of values, each stored in stride elements of
// the array.  If obj is not an array, isList is false and other is the
// resolved object.  If the array is malformed, err is a malformed file
// error and the values give the number of entries the array would hold.
func (lc listCache[T]) get(c pdf.Cursor, obj pdf.Object, stride int, decode func(pdf.Array) (T, error)) (values []T, isList bool, other pdf.Native, err error) {
	var chain []pdf.Reference
	var l *cachedList[T]
	for l == nil {
		ref, isRef := obj.(pdf.Reference)
		if !isRef {
			var native pdf.Native
			if obj != nil {
				native = obj.AsPDF(0)
			}
			l, err = decodeList(native, stride, decode)
			if err != nil {
				return nil, false, nil, err
			}
			break
		}
		if cached, ok := lc[ref]; ok {
			l = cached
			if l == nil { // ref is on the current chain
				l = &cachedList[T]{err: &pdf.MalformedFileError{Err: pdf.ErrCycle}}
			}
		} else {
			lc[ref] = nil
			chain = append(chain, ref)
			obj, err = c.Getter().Get(ref, true)
			if err != nil {
				return nil, false, nil, err
			}
		}
	}
	for _, ref := range chain {
		lc[ref] = l
	}
	return l.values, l.isList, l.other, l.err
}

// decodeList implements the decoding step of [listCache.get] for a resolved
// object.  Only read errors are returned directly.
func decodeList[T any](obj pdf.Native, stride int, decode func(pdf.Array) (T, error)) (*cachedList[T], error) {
	a, ok := obj.(pdf.Array)
	if !ok {
		return &cachedList[T]{other: obj}, nil
	}

	l := &cachedList[T]{values: make([]T, len(a)/stride), isList: true}
	if len(a)%stride != 0 {
		l.err = &pdf.MalformedFileError{Err: errMalformedList}
	}
	for i := range l.values {
		if l.err != nil {
			break
		}
		l.values[i], l.err = decode(a[stride*i : stride*i+stride])
		if pdf.IsReadError(l.err) {
			return nil, l.err
		}
	}
	return l, nil
}

// The errors below are wrapped in a new [pdf.MalformedFileError] for every
// use, since [pdf.Wrap] records the location in the error value itself.
var (
	errMalformedList = errors.New("malformed metrics array")
	errNonFinite     = errors.New("non-finite number")
)

// decodeVDefault reads default vertical metrics from a composite font's DW2
// entry.
//
// The DW2 entry is optional.  If it is not an array starting with two
// numbers, the result is [dict.DefaultVMetricsDefault].  Only read errors are
// returned.
func decodeVDefault(c pdf.Cursor, obj pdf.Object) (dict.DefaultVMetrics, error) {
	a, err := c.Array(obj)
	if pdf.IsReadError(err) {
		return dict.DefaultVMetrics{}, err
	} else if len(a) < 2 {
		return dict.DefaultVMetricsDefault, nil
	}

	offsY, err := number(c, a[0])
	if err != nil {
		return dropVDefault(err)
	}
	deltaY, err := number(c, a[1])
	if err != nil {
		return dropVDefault(err)
	}
	return dict.DefaultVMetrics{OffsY: offsY, DeltaY: deltaY}, nil
}

func dropVDefault(err error) (dict.DefaultVMetrics, error) {
	if pdf.IsReadError(err) {
		return dict.DefaultVMetrics{}, err
	}
	return dict.DefaultVMetricsDefault, nil
}

// decodeVMetrics reads vertical metrics information from a composite font's
// W2 array and returns a map from CID to vertical metrics.
//
// The W2 entry is optional.  If it is not an array, the result is nil.  If
// the array is malformed, the groups before the first malformed one are kept.
// If groups overlap, the first group which covers a CID gives its metrics.
// Only read errors are returned.
func decodeVMetrics(c pdf.Cursor, obj pdf.Object) (map[cid.CID]dict.VMetrics, error) {
	a, err := c.Array(obj)
	if pdf.IsReadError(err) {
		return nil, err
	} else if a == nil {
		return nil, nil
	}

	type group struct {
		cidRange
		v  dict.VMetrics   // for the range format
		vv []dict.VMetrics // for the list format
	}
	var groups []group
	lists := make(listCache[dict.VMetrics])
	triple := func(a pdf.Array) (dict.VMetrics, error) { return vMetrics(c, a) }
groupLoop:
	for len(a) > 1 {
		x, err := c.Integer(a[0])
		if err != nil {
			if pdf.IsReadError(err) {
				return nil, err
			}
			break
		}
		if x < 0 || x > maxCID {
			break
		}
		var g group
		g.first = int(x)

		vv, isList, other, err := lists.get(c, a[1], 3, triple)
		switch {
		case pdf.IsReadError(err):
			return nil, err
		case isList: // individual format: cid [dy1 ox1 oy1 ...]
			if int(x)+len(vv) > maxCID+1 || err != nil {
				break groupLoop
			}
			g.last = int(x) + len(vv) - 1
			g.vv = vv
			a = a[2:]
		default: // range format: cid1 cid2 dy ox oy
			x1, err := c.Integer(other)
			if err != nil || len(a) < 5 || x1 < x || x1 > maxCID {
				break groupLoop
			}
			g.v, err = vMetrics(c, a[2:5])
			if pdf.IsReadError(err) {
				return nil, err
			} else if err != nil {
				break groupLoop
			}
			g.last = int(x1)
			a = a[5:]
		}
		groups = append(groups, g)
	}

	// an empty W2 array is never written, so it reads as nil
	var res map[cid.CID]dict.VMetrics
	ranges := make([]cidRange, len(groups))
	for k, g := range groups {
		ranges[k] = g.cidRange
	}
	earliest(ranges, func(k, c int) {
		if res == nil {
			res = make(map[cid.CID]dict.VMetrics)
		}
		g := &groups[k]
		if g.vv != nil {
			res[cid.CID(c)] = g.vv[c-g.first]
		} else {
			res[cid.CID(c)] = g.v
		}
	})
	return res, nil
}

// vMetrics reads one W2 triple dy, ox, oy.
func vMetrics(c pdf.Cursor, a pdf.Array) (dict.VMetrics, error) {
	dy, err := number(c, a[0])
	if err != nil {
		return dict.VMetrics{}, err
	}
	offsX, err := number(c, a[1])
	if err != nil {
		return dict.VMetrics{}, err
	}
	offsY, err := number(c, a[2])
	if err != nil {
		return dict.VMetrics{}, err
	}
	return dict.VMetrics{DeltaY: dy, OffsX: offsX, OffsY: offsY}, nil
}

// number reads a finite number.  Non-finite values cannot be written to a
// PDF file, and are reported as malformed.
func number(c pdf.Cursor, obj pdf.Object) (float64, error) {
	x, err := c.Number(obj)
	if err != nil {
		return 0, err
	}
	if math.IsInf(x, 0) || math.IsNaN(x) {
		return 0, &pdf.MalformedFileError{Err: errNonFinite}
	}
	return x, nil
}
