// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package cidenc

import (
	"errors"
	"iter"
	"sync"

	"golang.org/x/text/unicode/norm"

	"seehuhn.de/go/postscript/cid"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/mapping"
)

var _ CIDEncoder = (*compositeUTF8)(nil)

type compositeUTF8 struct {
	wMode font.WritingMode

	codec     *charcode.Codec
	cid0Width float64
	cid0V     VMetrics

	// implied is the text which the character collection of the font
	// implies for each CID.  A reader uses it for codes whose ToUnicode
	// entry is empty.
	implied map[cid.CID]string

	// mu guards the fields below.  Readers hold it for one lookup at a time,
	// never across a yield.
	mu          sync.RWMutex
	info        map[charcode.Code]*codeInfo
	code        map[key]charcode.Code
	nextPrivate rune

	// metrics holds the width and the vertical metrics of every CID in use,
	// and of CID 0.  A CID has one set of metrics, shared by all its codes.
	metrics map[cid.CID]cidMetrics
}

// cidMetrics is the width and the vertical metrics of a CID.
type cidMetrics struct {
	width float64
	v     VMetrics
}

// NewCompositeUtf8 creates a CIDEncoder which allocates UTF-8 codes as
// glyphs are used.  The font uses the character collection ros, whose
// registry and ordering determine the text a CID implies; ros may be nil for
// a collection which implies no text.  Codes not allocated map to CID 0, with
// metrics cid0Width and cid0V.
func NewCompositeUtf8(ros *cid.SystemInfo, cid0Width float64, cid0V VMetrics, wMode font.WritingMode) CIDEncoder {
	codec, err := charcode.NewCodec(charcode.UTF8)
	if err != nil {
		panic(err)
	}
	var implied map[cid.CID]string
	if ros != nil {
		implied, _ = mapping.GetCIDTextMapping(ros.Registry, ros.Ordering)
	}
	e := &compositeUTF8{
		implied:     implied,
		wMode:       wMode,
		codec:       codec,
		info:        make(map[charcode.Code]*codeInfo),
		cid0Width:   cid0Width,
		cid0V:       cid0V,
		code:        make(map[key]charcode.Code),
		nextPrivate: 0x00_E000,
		metrics:     map[cid.CID]cidMetrics{0: {cid0Width, cid0V}},
	}
	return e
}

func (e *compositeUTF8) WritingMode() font.WritingMode {
	return e.wMode
}

func (e *compositeUTF8) Codes(s pdf.String) iter.Seq[font.Code] {
	vertical := e.wMode == font.Vertical
	return func(yield func(font.Code) bool) {
		var code font.Code
		for len(s) > 0 {
			c, k, valid := e.codec.Decode(s)

			var info *codeInfo
			if valid {
				info = e.lookup(c)
			}
			// The CMap written for this encoder has no notdef mappings, so
			// a reader selects CID 0 for every code not allocated, whether
			// valid or not, and Notdef is always 0.
			v := e.cid0V
			if info != nil { // code is mapped to a CID
				code.CID = info.CID
				code.Width = info.Width / 1000
				code.Text = info.Text
				if code.Text == "" {
					code.Text = e.implied[info.CID]
				}
				v = info.VMetrics
			} else { // unmapped or invalid code
				code.CID = 0
				code.Width = e.cid0Width / 1000
				code.Text = ""
				if valid {
					code.Text = e.implied[0]
				}
			}
			if vertical {
				code.VerticalAdvance = v.DeltaY / 1000
				code.VerticalOriginX = code.Width / 2
				code.VerticalOriginY = v.OffsY / 1000
			}

			code.UseWordSpacing = (k == 1 && c == 0x20)

			if !yield(code) {
				break
			}

			s = s[k:]
		}
	}
}

func (e *compositeUTF8) Codec() *charcode.Codec {
	return e.codec
}

func (e *compositeUTF8) GetCode(cid cid.CID, text string) (charcode.Code, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	key := key{cid, text}
	code, ok := e.code[key]
	return code, ok
}

// Encode returns the code for a CID and text, allocating a new one on first
// use.  The metrics are stored with the first code of a CID; every later
// call for the CID must give the same metrics.
func (e *compositeUTF8) Encode(cidVal cid.CID, text string, width float64, v VMetrics) (charcode.Code, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// all conflicts are checked before any map is written, so a rejected
	// call leaves the encoder unchanged
	m, hasMetrics := e.metrics[cidVal]
	if hasMetrics && m.width != width {
		return 0, errors.New("width already set to different value")
	}
	// vertical metrics are not used in horizontal writing mode
	if e.wMode == font.Vertical && hasMetrics && m.v != v {
		return 0, errors.New("vertical metrics already set to different values")
	}

	key := key{cidVal, text}
	if code, ok := e.code[key]; ok {
		return code, nil
	}

	code, err := e.makeCode(text)
	if err != nil {
		return 0, err
	}

	if !hasMetrics {
		m = cidMetrics{width, v}
		e.metrics[cidVal] = m
	}
	e.info[code] = &codeInfo{CID: cidVal, Width: m.width, VMetrics: m.v, Text: text}
	e.code[key] = code

	return code, nil
}

func (e *compositeUTF8) makeCode(text string) (charcode.Code, error) {
	if rr := []rune(norm.NFC.String(text)); len(rr) == 1 {
		code := runeToCode(rr[0])
		if _, alreadUsed := e.info[code]; !alreadUsed {
			return code, nil
		}
	}

	for {
		r := e.nextPrivate
		e.nextPrivate++
		switch e.nextPrivate {
		case 0x00_F900:
			e.nextPrivate = 0x0F_0000
		case 0x0F_FFFE:
			e.nextPrivate = 0x10_0000
		case 0x10_FFFE:
			return 0, ErrOverflow
		}
		code := runeToCode(r)
		if _, alreadUsed := e.info[code]; !alreadUsed {
			return code, nil
		}
	}
}

func runeToCode(r rune) charcode.Code {
	var code charcode.Code
	for i, b := range []byte(string(r)) {
		code |= charcode.Code(b) << (8 * i)
	}
	return code
}

// lookup returns the entry for a code, or nil for a code not allocated.  The
// entry stays valid after the lock is released, because entries are never
// changed once stored.
func (e *compositeUTF8) lookup(c charcode.Code) *codeInfo {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.info[c]
}

func (e *compositeUTF8) CMap(ros *cid.SystemInfo) *cmap.File {
	e.mu.RLock()
	m := make(map[charcode.Code]cid.CID, len(e.info))
	for c, info := range e.info {
		m[c] = info.CID
	}
	e.mu.RUnlock()
	cmapInfo := &cmap.File{
		Name:  "",
		ROS:   ros,
		WMode: e.wMode,
	}
	// The UTF-8 code space ranges do not overlap.
	_ = cmapInfo.SetMapping(e.Codec(), m)
	cmapInfo.UpdateName()
	return cmapInfo
}

func (e *compositeUTF8) Width(c charcode.Code) float64 {
	if info := e.lookup(c); info != nil {
		return info.Width
	}
	return e.cid0Width
}

func (e *compositeUTF8) MappedCodes() iter.Seq2[charcode.Code, *Info] {
	return func(yield func(charcode.Code, *Info) bool) {
		// the map is walked under the lock and yielded from outside it
		e.mu.RLock()
		type entry struct {
			c    charcode.Code
			info *codeInfo
		}
		entries := make([]entry, 0, len(e.info))
		for c, info := range e.info {
			entries = append(entries, entry{c, info})
		}
		e.mu.RUnlock()

		var code Info
		for _, en := range entries {
			code.CID = en.info.CID
			code.Width = en.info.Width
			code.VMetrics = en.info.VMetrics
			code.Text = en.info.Text
			if !yield(en.c, &code) {
				return
			}
		}
	}
}

func (e *compositeUTF8) ToUnicode() *cmap.ToUnicodeFile {
	e.mu.RLock()
	m := make(map[charcode.Code]string, len(e.info))
	for c, info := range e.info {
		m[c] = info.Text
	}
	e.mu.RUnlock()

	toUnicode, err := cmap.NewToUnicodeFile(charcode.UTF8, m)
	if err != nil {
		panic("unreachable")
	}

	return toUnicode
}

// CID0Metrics returns the metrics of CID 0, as given to [NewCompositeUtf8].
func (e *compositeUTF8) CID0Metrics() (float64, VMetrics) {
	return e.cid0Width, e.cid0V
}

func (e *compositeUTF8) CodesRemaining() int {
	const num_surrogates = 0xDFFF - 0xD800 + 1
	const num_range = 0x10_FFFF - 0x00_0000 + 1
	e.mu.RLock()
	defer e.mu.RUnlock()
	return num_range - num_surrogates - len(e.info)
}
