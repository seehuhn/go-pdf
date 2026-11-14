// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package cidenc

import (
	"errors"
	"iter"
	"maps"
	"sync"

	"seehuhn.de/go/postscript/cid"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/mapping"
)

// fixed represents a CIDEncoder that uses a fixed CMap.
// No new codes can be allocated.
// Text and glyph metrics can be added.
type fixed struct {
	cmap  *cmap.File
	codec *charcode.Codec

	// csr is the code space range of the CMap, including the ranges inherited
	// through usecmap in their original order, with overlaps removed.
	csr charcode.CodeSpaceRange

	all map[cid.CID]charcode.Code
	rev map[charcode.Code]cid.CID

	// implied is the text which the character collection of the font
	// implies for each CID.  A reader uses it for codes without a
	// ToUnicode entry.
	implied map[cid.CID]string

	// mu guards text, width, vMetrics and resolved, the maps which grow
	// while the encoder is used; the CMap and the maps derived from it in
	// NewFromCMap never change.  Encode sets width and vMetrics together, so
	// both have the same keys.
	mu       sync.RWMutex
	text     map[charcode.Code]string
	width    map[cid.CID]float64
	vMetrics map[cid.CID]VMetrics
	resolved map[codeKey]resolvedCode
}

// codeKey identifies a character code together with its length in bytes,
// since an incomplete code at the end of a string can have the same value as
// a longer code.
type codeKey struct {
	code charcode.Code
	n    int
}

// resolvedCode gives the CID a reader selects for a code, and the CID of its
// notdef mapping.
type resolvedCode struct {
	cid, notdef cid.CID
}

var _ CIDEncoder = (*fixed)(nil)

// NewFromCMap creates a CIDEncoder from an existing CMap.  The font uses the
// character collection ros, whose registry and ordering determine the text a
// CID implies; ros may be nil for a collection which implies no text.  The
// metrics of CID 0 are set to cid0Width and cid0V.
// This returns an error if [cmap.File.Codec] fails for the CMap: if the code
// space range is invalid, or if its lookup tree would be too large.
// It also fails for a CMap whose code space ranges are too complex to be
// written without overlaps.
func NewFromCMap(cmap *cmap.File, ros *cid.SystemInfo, cid0Width float64, cid0V VMetrics) (CIDEncoder, error) {
	codec, err := cmap.Codec()
	if err != nil {
		return nil, err
	}

	// the ranges are those of cmap.Codec, in the same order
	var cs charcode.CodeSpaceRange
	for _, g := range cmap.Chain() {
		cs = append(cs, g.CodeSpaceRange...)
	}
	csr, err := cs.Disjoint()
	if err != nil {
		return nil, err
	}
	// ToUnicode builds a codec from csr, so check here that this succeeds
	if _, err := charcode.NewCodec(csr); err != nil {
		return nil, err
	}

	// A code mapped by both the CMap and its parent belongs to the CMap,
	// so the last mapping seen for a code is the one which counts, and a
	// CID reachable only through a shadowed mapping gets no code.
	rev := maps.Collect(cmap.All(codec))

	// A CID reachable through several codes is written as the smallest of
	// them, so that the choice does not depend on map iteration order.
	all := make(map[cid.CID]charcode.Code)
	for code, c := range cmap.All(codec) {
		if rev[code] != c {
			continue
		}
		if prev, ok := all[c]; !ok || code < prev {
			all[c] = code
		}
	}

	var implied map[cid.CID]string
	if ros != nil {
		implied, _ = mapping.GetCIDTextMapping(ros.Registry, ros.Ordering)
	}

	return &fixed{
		cmap:     cmap,
		codec:    codec,
		csr:      csr,
		all:      all,
		rev:      rev,
		implied:  implied,
		text:     make(map[charcode.Code]string),
		width:    map[cid.CID]float64{0: cid0Width},
		vMetrics: map[cid.CID]VMetrics{0: cid0V},
		resolved: make(map[codeKey]resolvedCode),
	}, nil
}

// WritingMode indicates whether the font is for horizontal or vertical
// writing.
func (f *fixed) WritingMode() font.WritingMode {
	return f.cmap.WMode
}

// Codes iterates over the character codes in a PDF string.
//
// The iterator returns, for every code, the information a reader of the
// written font reports.  The CID and the notdef CID follow the CMap,
// including its notdef mappings.  A CID which was never passed to
// [fixed.Encode] has the metrics of CID 0, which the composite font writers
// record as the font's defaults.
func (f *fixed) Codes(s pdf.String) iter.Seq[font.Code] {
	vertical := f.cmap.WMode == font.Vertical
	return func(yield func(font.Code) bool) {
		for len(s) > 0 {
			c, k, valid := f.codec.Decode(s)
			r := f.resolve(s[:k], c, valid)

			f.mu.RLock()
			width, v := f.metrics(r.cid)
			var text string
			if valid {
				text = f.text[c]
			}
			f.mu.RUnlock()
			if valid && text == "" {
				text = f.implied[r.cid]
			}

			code := font.Code{
				CID:            r.cid,
				Notdef:         r.notdef,
				Width:          width / 1000,
				Text:           text,
				UseWordSpacing: valid && k == 1 && c == 0x20,
			}
			if vertical {
				code.VerticalAdvance = v.DeltaY / 1000
				code.VerticalOriginX = code.Width / 2
				code.VerticalOriginY = v.OffsY / 1000
			}

			if !yield(code) {
				return
			}
			s = s[k:]
		}
	}
}

// resolve returns the CID and the notdef CID for the code c, which consists
// of the given bytes.  A valid code selects its CID mapping, or its notdef
// mapping if there is none; an invalid code selects its notdef mapping.
func (f *fixed) resolve(codeBytes []byte, c charcode.Code, valid bool) resolvedCode {
	key := codeKey{c, len(codeBytes)}
	f.mu.RLock()
	r, ok := f.resolved[key]
	f.mu.RUnlock()
	if ok {
		return r
	}

	if valid {
		r.cid = f.cmap.LookupCID(codeBytes)
		r.notdef = f.cmap.LookupNotdefCID(codeBytes)
	} else {
		r.cid = f.cmap.LookupNotdefCID(codeBytes)
	}

	f.mu.Lock()
	f.resolved[key] = r
	f.mu.Unlock()
	return r
}

// metrics returns the width and the vertical metrics stored for a CID, or
// those of CID 0 if the CID has not been encoded.  The caller must hold
// f.mu.
func (f *fixed) metrics(cidVal cid.CID) (float64, VMetrics) {
	if w, ok := f.width[cidVal]; ok {
		return w, f.vMetrics[cidVal]
	}
	return f.width[0], f.vMetrics[0]
}

// MappedCodes iterates over all codes known to the encoder.
func (f *fixed) MappedCodes() iter.Seq2[charcode.Code, *Info] {
	return func(yield func(charcode.Code, *Info) bool) {
		// the maps are walked under the lock and yielded from outside it
		f.mu.RLock()
		type entry struct {
			code charcode.Code
			info Info
		}
		entries := make([]entry, 0, len(f.text))
		for code, text := range f.text {
			cid := f.rev[code]
			entries = append(entries, entry{code, Info{
				CID:      cid,
				Width:    f.width[cid],
				VMetrics: f.vMetrics[cid],
				Text:     text,
			}})
		}
		f.mu.RUnlock()

		var info Info
		for _, e := range entries {
			info = e.info
			if !yield(e.code, &info) {
				break
			}
		}
	}
}

// Encode returns the code the CMap gives a CID, storing the text and the
// metrics with it on first use.
func (f *fixed) Encode(cidVal cid.CID, text string, width float64, v VMetrics) (charcode.Code, error) {
	code, ok := f.all[cidVal]
	if !ok {
		return 0, errors.New("CID not found in CMap")
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	// all conflicts are checked before any map is written, so a rejected
	// call leaves the encoder unchanged
	existingWidth, hasMetrics := f.width[cidVal]
	if hasMetrics && existingWidth != width {
		return 0, errors.New("width already set to different value")
	}
	// vertical metrics are not used in horizontal writing mode
	vertical := f.cmap.WMode == font.Vertical
	if vertical && hasMetrics && f.vMetrics[cidVal] != v {
		return 0, errors.New("vertical metrics already set to different values")
	}
	existingText, hasText := f.text[code]
	if hasText && existingText != text {
		return 0, errors.New("text already set to different value")
	}

	if !hasMetrics {
		f.width[cidVal] = width
		f.vMetrics[cidVal] = v
	}
	if !hasText {
		f.text[code] = text
	}

	return code, nil
}

// CMap returns the underlying CMap.
func (f *fixed) CMap(ros *cid.SystemInfo) *cmap.File {
	return f.cmap
}

// Codec returns the character code codec.
func (f *fixed) Codec() *charcode.Codec {
	return f.codec
}

// GetCode returns the code the CMap gives a CID, if the CID has been
// encoded.  The metrics of CID 0 are known from the start, but CID 0 has a
// code only if the CMap maps one to it.
func (f *fixed) GetCode(cidVal cid.CID, text string) (charcode.Code, bool) {
	code, ok := f.all[cidVal]
	if !ok {
		return 0, false
	}
	f.mu.RLock()
	defer f.mu.RUnlock()
	if _, ok := f.width[cidVal]; !ok {
		return 0, false
	}
	return code, true
}

// Width returns the width of the given valid character code: the width of
// the CID a reader selects for it, or the width of CID 0 if that CID has not
// been encoded.
func (f *fixed) Width(code charcode.Code) float64 {
	r := f.resolve(f.codec.AppendCode(nil, code), code, true)
	f.mu.RLock()
	defer f.mu.RUnlock()
	w, _ := f.metrics(r.cid)
	return w
}

// ToUnicode returns a ToUnicode CMap representing the text content
// of the mapped codes.
func (f *fixed) ToUnicode() *cmap.ToUnicodeFile {
	m := make(map[charcode.Code]string)

	f.mu.RLock()
	defer f.mu.RUnlock()

	var buf []byte
	for code, text := range f.text {
		if text == "" {
			continue
		}

		buf = f.codec.AppendCode(buf[:0], code)
		cidVal := f.cmap.LookupCID(buf)
		if text == f.implied[cidVal] {
			continue
		}

		m[code] = text
	}

	if len(m) == 0 {
		return nil
	}

	// The code space includes the ranges inherited through usecmap, as for
	// Identity-V, and keeps them in their original order.  NewFromCMap
	// checked that a codec for csr exists, so this cannot fail.
	toUnicode, _ := cmap.NewToUnicodeFile(f.csr, m)
	return toUnicode
}

// CID0Metrics returns the metrics of CID 0, as given to [NewFromCMap].
func (f *fixed) CID0Metrics() (float64, VMetrics) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.width[0], f.vMetrics[0]
}

func (f *fixed) CodesRemaining() int {
	return 0
}
