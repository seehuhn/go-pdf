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

package jbig2

// MQ arithmetic coder for JBIG2 (ITU-T T.88, Annex E).
//
// Contexts are packed into a single byte: bits 1-7 = probability estimation
// index (0-46), bit 0 = MPS value.
//
// This implementation uses the standard Annex E convention:
//   MPS = lower sub-interval [C, C+A)
//   LPS = upper sub-interval [C+A, C+A+Qe)
//   Encoder MPS: C unchanged (stays in lower)
//   Encoder LPS: C += A (move to upper)
//   Decoder: Chigh >= A → LPS (subtract A), Chigh < A → MPS

type qeEntry struct {
	qe   uint16
	nmps byte
	nlps byte
	sw   bool
}

var qeTable = [47]qeEntry{
	{0x5601, 1, 1, true}, {0x3401, 2, 6, false}, {0x1801, 3, 9, false},
	{0x0AC1, 4, 12, false}, {0x0521, 5, 29, false}, {0x0221, 38, 33, false},
	{0x5601, 7, 6, true}, {0x5401, 8, 14, false}, {0x4801, 9, 14, false},
	{0x3801, 10, 14, false}, {0x3001, 11, 17, false}, {0x2401, 12, 18, false},
	{0x1C01, 13, 20, false}, {0x1601, 29, 21, false}, {0x5601, 15, 14, true},
	{0x5401, 16, 14, false}, {0x5101, 17, 15, false}, {0x4801, 18, 16, false},
	{0x3801, 19, 17, false}, {0x3401, 20, 18, false}, {0x3001, 21, 19, false},
	{0x2801, 22, 19, false}, {0x2401, 23, 20, false}, {0x2201, 24, 21, false},
	{0x1C01, 25, 22, false}, {0x1801, 26, 23, false}, {0x1601, 27, 24, false},
	{0x1401, 28, 25, false}, {0x1201, 29, 26, false}, {0x1101, 30, 27, false},
	{0x0AC1, 31, 28, false}, {0x09C1, 32, 29, false}, {0x08A1, 33, 30, false},
	{0x0521, 34, 31, false}, {0x0441, 35, 32, false}, {0x02A1, 36, 33, false},
	{0x0221, 37, 34, false}, {0x0141, 38, 35, false}, {0x0111, 39, 36, false},
	{0x0085, 40, 37, false}, {0x0049, 41, 38, false}, {0x0025, 42, 39, false},
	{0x0015, 43, 40, false}, {0x0009, 44, 41, false}, {0x0005, 45, 42, false},
	{0x0001, 45, 43, false}, {0x5601, 46, 46, false},
}

func ctxIndex(cx byte) byte { return cx >> 1 }
func ctxMPS(cx byte) int    { return int(cx & 1) }
func ctxSet(idx byte, mps int) byte {
	return idx<<1 | byte(mps)
}

// --- Encoder (Annex E) ---

type mqEncoder struct {
	a   uint16
	c   uint32
	ct  int
	buf []byte
	bp  int
}

func newMQEncoder() *mqEncoder {
	return &mqEncoder{a: 0x8000, ct: 12, buf: []byte{0}}
}

func (e *mqEncoder) encode(cx *byte, d int) {
	idx := ctxIndex(*cx)
	mps := ctxMPS(*cx)
	qe := qeTable[idx].qe
	e.a -= qe
	if d == mps {
		e.codeMPS(cx, idx, qe)
	} else {
		e.codeLPS(cx, idx, qe)
	}
}

func (e *mqEncoder) codeMPS(cx *byte, idx byte, qe uint16) {
	// Annex G encoder: MPS → C += Qe (or exchange: A = Qe)
	if e.a&0x8000 == 0 {
		if e.a < qe {
			e.a = qe
		} else {
			e.c += uint32(qe)
		}
		*cx = ctxSet(qeTable[idx].nmps, ctxMPS(*cx))
		e.renorme()
	} else {
		e.c += uint32(qe)
	}
}

func (e *mqEncoder) codeLPS(cx *byte, idx byte, qe uint16) {
	// Annex G encoder: LPS exchange → C += Qe; no exchange → A = Qe
	if e.a < qe {
		e.c += uint32(qe)
	} else {
		e.a = qe
	}
	mps := ctxMPS(*cx)
	if qeTable[idx].sw {
		mps ^= 1
	}
	*cx = ctxSet(qeTable[idx].nlps, mps)
	e.renorme()
}

func (e *mqEncoder) renorme() {
	for e.a < 0x8000 {
		e.a <<= 1
		e.c <<= 1
		e.ct--
		if e.ct == 0 {
			e.byteOut()
		}
	}
}

func (e *mqEncoder) byteOut() {
	if e.buf[e.bp] == 0xFF {
		e.bp++
		e.buf = append(e.buf, byte(e.c>>20))
		e.c &= 0xFFFFF
		e.ct = 7
	} else if e.c&0x08000000 != 0 {
		e.buf[e.bp]++
		e.c &= 0x7FFFFFF
		if e.buf[e.bp] == 0xFF {
			e.bp++
			e.buf = append(e.buf, byte(e.c>>20))
			e.c &= 0xFFFFF
			e.ct = 7
		} else {
			e.bp++
			e.buf = append(e.buf, byte(e.c>>19))
			e.c &= 0x7FFFF
			e.ct = 8
		}
	} else {
		e.bp++
		e.buf = append(e.buf, byte(e.c>>19))
		e.c &= 0x7FFFF
		e.ct = 8
	}
}

func (e *mqEncoder) flush() {
	// T.88 E.2.9 FLUSH: SETBITS then drain C via BYTEOUT.
	temp := e.c + uint32(e.a)
	e.c |= 0xFFFF
	if e.c >= temp {
		e.c -= 0x8000
	}

	// first two BYTEOUTs
	e.c <<= uint(e.ct)
	e.byteOut()
	e.c <<= uint(e.ct)
	e.byteOut()

	// After a 0xFF byte, BYTEOUT extracts only 7 bits instead of 8,
	// leaving residual data in C. A 3rd BYTEOUT flushes these bits.
	// When B is already 0xFF, the residual is handled by the final
	// BYTEOUT below instead.
	if e.buf[e.bp] != 0xFF {
		e.c <<= uint(e.ct)
		e.byteOut()
	}

	// final BYTEOUT emits the buffered byte B; the byte it creates
	// is not part of the output (bytes() excludes it).
	e.c <<= uint(e.ct)
	e.byteOut()
}

func (e *mqEncoder) bytes() []byte {
	if e.bp < 2 {
		return nil
	}
	out := e.buf[1:e.bp]
	// The FLUSH procedure (Figure E.11) ends by appending the
	// terminating marker code 0xFF 0xAC.  The 0xFF overlaps the
	// final coded data; append 0xAC to complete the marker.
	if len(out) > 0 && out[len(out)-1] == 0xFF {
		out = append(out, 0xAC)
	}
	return out
}

// --- Decoder (Annex E) ---

type mqDecoder struct {
	a    uint16
	c    uint32
	ct   int
	data []byte
	bp   int
	b    byte

	// exhausted is set once the real data has been consumed.
	// After exhaustion, byteIn supplies 1-bits per T.88 E.2.10.
	exhausted    bool
	paddingCount int
}

// maxMQPadding limits decode() calls after data exhaustion.
// When exceeded, decode returns MPS directly — this matches what
// the 1-bit padding would produce and is correct for legitimate data.
const maxMQPadding = 1 << 20

func newMQDecoder(data []byte) *mqDecoder {
	d := &mqDecoder{data: data}
	d.initDec()
	return d
}

func (d *mqDecoder) initDec() {
	if len(d.data) == 0 {
		d.exhausted = true
		d.a = 0x8000
		return
	}
	d.bp = 0
	d.b = d.data[0]
	d.c = uint32(d.b) << 16
	d.byteIn()
	d.c <<= 7
	d.ct -= 7
	d.a = 0x8000
}

func (d *mqDecoder) decode(cx *byte) int {
	if d.exhausted {
		d.paddingCount++
		if d.paddingCount > maxMQPadding {
			return ctxMPS(*cx)
		}
	}

	idx := ctxIndex(*cx)
	mps := ctxMPS(*cx)
	qe := qeTable[idx].qe
	d.a -= qe

	if uint16(d.c>>16) < qe {
		return d.lpsExchange(cx, idx, mps, qe)
	}
	d.c -= uint32(qe) << 16
	if d.a&0x8000 == 0 {
		return d.mpsExchange(cx, idx, mps, qe)
	}
	return mps
}

func (d *mqDecoder) mpsExchange(cx *byte, idx byte, mps int, qe uint16) int {
	var result int
	if d.a < qe {
		// exchange
		result = 1 - mps
		newMPS := mps
		if qeTable[idx].sw {
			newMPS ^= 1
		}
		*cx = ctxSet(qeTable[idx].nlps, newMPS)
	} else {
		result = mps
		*cx = ctxSet(qeTable[idx].nmps, mps)
	}
	d.renormd()
	return result
}

func (d *mqDecoder) lpsExchange(cx *byte, idx byte, mps int, qe uint16) int {
	var result int
	if d.a < qe {
		// exchange
		result = mps
		*cx = ctxSet(qeTable[idx].nmps, mps)
	} else {
		result = 1 - mps
		newMPS := mps
		if qeTable[idx].sw {
			newMPS ^= 1
		}
		*cx = ctxSet(qeTable[idx].nlps, newMPS)
	}
	d.a = qe
	d.renormd()
	return result
}

func (d *mqDecoder) renormd() {
	for d.a < 0x8000 {
		if d.ct == 0 {
			d.byteIn()
		}
		d.a <<= 1
		d.c <<= 1
		d.ct--
	}
}

func (d *mqDecoder) byteIn() {
	// T.88 E.2.10: once data is exhausted (end of data or marker
	// encountered), supply 1-bits without bit stuffing.
	if d.exhausted {
		d.c += 0xFF << 8
		d.ct = 8
		return
	}
	if d.b == 0xFF {
		if d.bp+1 >= len(d.data) {
			d.exhausted = true
			d.c += 0xFF << 8
			d.ct = 8
			return
		}
		b1 := d.data[d.bp+1]
		if b1 > 0x8F {
			d.exhausted = true
			d.c += 0xFF << 8
			d.ct = 8
		} else {
			d.bp++
			d.b = d.data[d.bp]
			d.c += uint32(d.b) << 9
			d.ct = 7
		}
	} else {
		d.bp++
		if d.bp >= len(d.data) {
			d.exhausted = true
			d.c += 0xFF << 8
			d.ct = 8
			return
		}
		d.b = d.data[d.bp]
		d.c += uint32(d.b) << 8
		d.ct = 8
	}
}
