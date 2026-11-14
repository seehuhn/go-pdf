// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package cmap

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/postscript/cid"

	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/internal/limits"
)

// refLookupCID is the linear reference implementation of [File.LookupCID].
func refLookupCID(f *File, code []byte) cid.CID {
	for g := f; g != nil; g = g.Parent {
		for _, s := range slices.Backward(g.CIDSingles) {
			if bytes.Equal(s.Code, code) {
				return s.Value
			}
		}
		for _, r := range slices.Backward(g.CIDRanges) {
			if index, ok := rangeIndex(r.First, r.Last, code); ok && uint64(r.Value)+uint64(index) <= math.MaxUint32 {
				return r.Value + cid.CID(index)
			}
		}
	}
	return refLookupNotdefCID(f, code)
}

// refLookupNotdefCID is the linear reference implementation of
// [File.LookupNotdefCID].
func refLookupNotdefCID(f *File, code []byte) cid.CID {
	for g := f; g != nil; g = g.Parent {
		for _, s := range slices.Backward(g.NotdefSingles) {
			if bytes.Equal(s.Code, code) {
				return s.Value
			}
		}
	rangesLoop:
		for _, r := range slices.Backward(g.NotdefRanges) {
			if len(r.First) != len(code) || len(r.Last) != len(code) {
				continue
			}
			for i, b := range code {
				if b < r.First[i] || b > r.Last[i] {
					continue rangesLoop
				}
			}
			return r.Value
		}
	}
	return 0
}

// refToUnicodeLookup is the linear reference implementation of
// [ToUnicodeFile.Lookup].
func refToUnicodeLookup(tu *ToUnicodeFile, code []byte) (string, bool) {
	for g := tu; g != nil; g = g.Parent {
		for _, s := range slices.Backward(g.Singles) {
			if bytes.Equal(s.Code, code) {
				return s.Value, true
			}
		}
		for _, r := range slices.Backward(g.Ranges) {
			if len(r.Values) == 0 {
				continue
			}
			index, ok := rangeIndex(r.First, r.Last, code)
			if !ok {
				continue
			}
			if index < len(r.Values) {
				return r.Values[index], true
			}
			return nextString(r.Values[0], index), true
		}
	}
	return "", false
}

// randomCodeGen generates codes from a small alphabet, so that random
// entries overlap often.
type randomCodeGen struct {
	rng      *rand.Rand
	alphabet []byte
}

func newRandomCodeGen(rng *rand.Rand) *randomCodeGen {
	alphabet := []byte{0x00, 0x01, 0x02, 0x7F, 0x80, 0xFE, 0xFF}
	return &randomCodeGen{rng: rng, alphabet: alphabet}
}

func (g *randomCodeGen) byteVal() byte {
	return g.alphabet[g.rng.IntN(len(g.alphabet))]
}

func (g *randomCodeGen) code(l int) []byte {
	c := make([]byte, l)
	for i := range c {
		c[i] = g.byteVal()
	}
	return c
}

func (g *randomCodeGen) length() int {
	switch g.rng.IntN(20) {
	case 0:
		return 0
	case 1:
		return 5
	default:
		return 1 + g.rng.IntN(4)
	}
}

// pair returns the bounds of a random range.  Occasionally the range is
// invalid, or covers more than math.MaxInt32 codes.
func (g *randomCodeGen) pair() ([]byte, []byte) {
	l := g.length()
	first, last := g.code(l), g.code(l)
	switch g.rng.IntN(10) {
	case 0: // possibly invalid
	case 1: // lengths differ
		last = g.code(g.length())
	case 2: // huge
		first = bytes.Repeat([]byte{0x00}, 4)
		last = bytes.Repeat([]byte{0xFF}, 4)
		last[0] = g.byteVal()
	default:
		for i := range first {
			if first[i] > last[i] {
				first[i], last[i] = last[i], first[i]
			}
		}
		if g.rng.IntN(2) == 0 && l > 0 {
			// only the last byte varies
			copy(last[:l-1], first[:l-1])
		}
	}
	return first, last
}

func randomFile(g *randomCodeGen, depth int) *File {
	f := &File{}
	for range g.rng.IntN(8) {
		f.CIDSingles = append(f.CIDSingles, Single{Code: g.code(g.length()), Value: cid.CID(g.rng.IntN(1000))})
	}
	for range g.rng.IntN(8) {
		first, last := g.pair()
		value := cid.CID(g.rng.IntN(1000))
		if g.rng.IntN(4) == 0 {
			// near the largest CID, so that the range overflows
			value = math.MaxUint32 - value
		}
		f.CIDRanges = append(f.CIDRanges, Range{First: first, Last: last, Value: value})
	}
	for range g.rng.IntN(4) {
		f.NotdefSingles = append(f.NotdefSingles, Single{Code: g.code(g.length()), Value: cid.CID(g.rng.IntN(1000))})
	}
	for range g.rng.IntN(4) {
		first, last := g.pair()
		f.NotdefRanges = append(f.NotdefRanges, Range{First: first, Last: last, Value: cid.CID(g.rng.IntN(1000))})
	}
	if depth > 0 && g.rng.IntN(2) == 0 {
		f.Parent = randomFile(g, depth-1)
	}
	return f
}

func randomToUnicode(g *randomCodeGen, depth int) *ToUnicodeFile {
	tu := &ToUnicodeFile{}
	for range g.rng.IntN(8) {
		tu.Singles = append(tu.Singles, ToUnicodeSingle{Code: g.code(g.length()), Value: string(rune('a' + g.rng.IntN(26)))})
	}
	for range g.rng.IntN(8) {
		first, last := g.pair()
		var values []string
		for range g.rng.IntN(4) {
			values = append(values, string(rune('A'+g.rng.IntN(26))))
		}
		tu.Ranges = append(tu.Ranges, ToUnicodeRange{First: first, Last: last, Values: values})
	}
	if depth > 0 && g.rng.IntN(2) == 0 {
		tu.Parent = randomToUnicode(g, depth-1)
	}
	return tu
}

// testCodes returns codes which exercise the entries of a CMap: the bounds
// of all entries, and random codes from the alphabet.
func testCodes(g *randomCodeGen, bounds [][]byte) [][]byte {
	codes := [][]byte{{}}
	for _, b := range bounds {
		codes = append(codes, b)
		if len(b) > 0 {
			c := bytes.Clone(b)
			c[len(c)-1]++
			codes = append(codes, c)
		}
	}
	for range 200 {
		codes = append(codes, g.code(g.length()))
	}
	return codes
}

func fileBounds(f *File) [][]byte {
	var res [][]byte
	for g := f; g != nil; g = g.Parent {
		for _, s := range g.CIDSingles {
			res = append(res, s.Code)
		}
		for _, s := range g.NotdefSingles {
			res = append(res, s.Code)
		}
		for _, r := range g.CIDRanges {
			res = append(res, r.First, r.Last)
		}
		for _, r := range g.NotdefRanges {
			res = append(res, r.First, r.Last)
		}
	}
	return res
}

// TestLookupCIDIndex compares the indexed lookups with the linear reference
// implementations.
func TestLookupCIDIndex(t *testing.T) {
	g := newRandomCodeGen(rand.New(rand.NewPCG(1, 2)))
	for range 3000 {
		f := randomFile(g, 2)
		for _, code := range testCodes(g, fileBounds(f)) {
			if got, want := f.LookupCID(code), refLookupCID(f, code); got != want {
				t.Fatalf("LookupCID(% x) = %d, want %d", code, got, want)
			}
			if got, want := f.LookupNotdefCID(code), refLookupNotdefCID(f, code); got != want {
				t.Fatalf("LookupNotdefCID(% x) = %d, want %d", code, got, want)
			}
		}
	}
}

// TestToUnicodeLookupIndex compares the indexed lookup with the linear
// reference implementation.
func TestToUnicodeLookupIndex(t *testing.T) {
	g := newRandomCodeGen(rand.New(rand.NewPCG(3, 4)))
	for range 3000 {
		tu := randomToUnicode(g, 2)
		var bounds [][]byte
		for h := tu; h != nil; h = h.Parent {
			for _, s := range h.Singles {
				bounds = append(bounds, s.Code)
			}
			for _, r := range h.Ranges {
				bounds = append(bounds, r.First, r.Last)
			}
		}
		for _, code := range testCodes(g, bounds) {
			got, gotOK := tu.Lookup(code)
			want, wantOK := refToUnicodeLookup(tu, code)
			if got != want || gotOK != wantOK {
				t.Fatalf("Lookup(% x) = %q %t, want %q %t", code, got, gotOK, want, wantOK)
			}
		}
	}
}

// smallCodec is a code space with one- and two-byte codes, used to compare
// All with the lookups.
var smallCodec = func() *charcode.Codec {
	c, err := charcode.NewCodec(charcode.CodeSpaceRange{
		{Low: []byte{0x00}, High: []byte{0x7F}},
		{Low: []byte{0x80, 0x00}, High: []byte{0xFF, 0xFF}},
	})
	if err != nil {
		panic(err)
	}
	return c
}()

// overlappingCodeGen generates codes which are valid for smallCodec.
func overlappingCodeGen(rng *rand.Rand) func() []byte {
	return func() []byte {
		if rng.IntN(2) == 0 {
			return []byte{byte(rng.IntN(8))}
		}
		return []byte{0x80 + byte(rng.IntN(3)), byte(rng.IntN(8))}
	}
}

// overlappingPair returns a range of codes valid for smallCodec, which
// overlaps with other such ranges.
func overlappingPair(rng *rand.Rand) ([]byte, []byte) {
	a, b := byte(rng.IntN(8)), byte(rng.IntN(8))
	if rng.IntN(2) == 0 {
		return []byte{min(a, b)}, []byte{max(a, b)}
	}
	c, d := 0x80+byte(rng.IntN(3)), 0x80+byte(rng.IntN(3))
	return []byte{min(c, d), min(a, b)}, []byte{max(c, d), max(a, b)}
}

// TestToUnicodeAllAgreesWithLookup checks that All reports every code
// exactly once, with the text Lookup gives, for overlapping bfchar and
// bfrange entries and parent CMaps.
func TestToUnicodeAllAgreesWithLookup(t *testing.T) {
	rng := rand.New(rand.NewPCG(5, 6))
	code := overlappingCodeGen(rng)
	var random func(depth int) *ToUnicodeFile
	random = func(depth int) *ToUnicodeFile {
		tu := &ToUnicodeFile{}
		for range rng.IntN(10) {
			tu.Singles = append(tu.Singles, ToUnicodeSingle{Code: code(), Value: string(rune('a' + rng.IntN(26)))})
		}
		for range rng.IntN(10) {
			first, last := overlappingPair(rng)
			values := []string{string(rune('A' + rng.IntN(26)))}
			if rng.IntN(2) == 0 {
				values = append(values, "x", "y")
			}
			tu.Ranges = append(tu.Ranges, ToUnicodeRange{First: first, Last: last, Values: values})
		}
		if depth > 0 && rng.IntN(2) == 0 {
			tu.Parent = random(depth - 1)
		}
		return tu
	}

	for range 2000 {
		tu := random(2)
		seen := make(map[charcode.Code]bool)
		for c, text := range tu.All(smallCodec) {
			if seen[c] {
				t.Fatalf("code %x reported twice", c)
			}
			seen[c] = true
			want, ok := tu.Lookup(smallCodec.AppendCode(nil, c))
			if !ok || text != want {
				t.Fatalf("code %x: All gives %q, Lookup gives %q %t", c, text, want, ok)
			}
		}
		// every code which Lookup knows is reported by All
		for range 50 {
			s := code()
			if _, ok := tu.Lookup(s); ok {
				c, _, _ := smallCodec.Decode(s)
				if !seen[c] {
					t.Fatalf("code % x not reported by All", s)
				}
			}
		}
	}
}

// TestFileAllAgreesWithLookup checks that All reports every code exactly
// once, with the CID LookupCID gives.
func TestFileAllAgreesWithLookup(t *testing.T) {
	rng := rand.New(rand.NewPCG(7, 8))
	code := overlappingCodeGen(rng)
	var random func(depth int) *File
	random = func(depth int) *File {
		f := &File{}
		for range rng.IntN(10) {
			f.CIDSingles = append(f.CIDSingles, Single{Code: code(), Value: cid.CID(rng.IntN(100))})
		}
		for range rng.IntN(10) {
			first, last := overlappingPair(rng)
			f.CIDRanges = append(f.CIDRanges, Range{First: first, Last: last, Value: cid.CID(rng.IntN(100))})
		}
		if depth > 0 && rng.IntN(2) == 0 {
			f.Parent = random(depth - 1)
		}
		return f
	}

	for range 2000 {
		f := random(2)
		seen := make(map[charcode.Code]bool)
		for c, cidVal := range f.All(smallCodec) {
			if seen[c] {
				t.Fatalf("code %x reported twice", c)
			}
			seen[c] = true
			if want := f.LookupCID(smallCodec.AppendCode(nil, c)); cidVal != want {
				t.Fatalf("code %x: All gives %d, LookupCID gives %d", c, cidVal, want)
			}
		}
	}
}

// TestAllBudgetChildFirst checks that, when the enumeration budget runs
// out, the mappings of a CMap are reported in preference to those of its
// parent.
func TestAllBudgetChildFirst(t *testing.T) {
	codec, err := charcode.NewCodec(charcode.CodeSpaceRange{
		{Low: []byte{0x00, 0x00, 0x00}, High: []byte{0xFF, 0xFF, 0xFF}},
	})
	if err != nil {
		t.Fatal(err)
	}
	parent := &File{CIDRanges: []Range{
		{First: []byte{0x00, 0x00, 0x00}, Last: []byte{0xFF, 0xFF, 0xFF}, Value: 0},
	}}
	child := &File{
		CIDSingles: []Single{{Code: []byte{0xFF, 0xFF, 0xFF}, Value: 7}},
		Parent:     parent,
	}
	found := false
	for c, cidVal := range child.All(codec) {
		if c == 0xFFFFFF {
			found = true
			if cidVal != 7 {
				t.Errorf("CID %d, want 7", cidVal)
			}
		}
	}
	if !found {
		t.Error("the mapping of the child CMap is missing")
	}
}

// TestIndexedBoxes checks the parts of large ranges which are indexed.
func TestIndexedBoxes(t *testing.T) {
	rng := rand.New(rand.NewPCG(9, 10))
	for range 200 {
		first := []byte{byte(rng.IntN(4)), byte(rng.IntN(256)), byte(rng.IntN(256)), byte(rng.IntN(256))}
		last := []byte{0xFF - byte(rng.IntN(4)), 0xFF, byte(rng.IntN(256)), 0xFF}
		for i := range first {
			if first[i] > last[i] {
				first[i], last[i] = last[i], first[i]
			}
		}
		maxIndex := math.MaxInt32
		if rng.IntN(2) == 0 {
			maxIndex = rng.IntN(1 << 20)
		}
		boxes := indexedBoxes(nil, first, last, maxIndex, 0)
		for range 2000 {
			code := make([]byte, 4)
			for i := range code {
				code[i] = first[i] + byte(rng.IntN(int(last[i]-first[i])+1))
			}
			index, want := rangeIndex(first, last, code)
			want = want && index <= maxIndex
			n := 0
			for _, b := range boxes {
				if boxContains(b.first, b.last, code) {
					n++
				}
			}
			if (n == 1) != want || n > 1 {
				t.Fatalf("% x-% x: code % x in %d boxes, indexed %t", first, last, code, n, want)
			}
		}
	}
}

// TestCodeIndexScaling checks that building the index takes memory linear
// in the number of entries, and that a lookup inspects a number of
// entries which grows logarithmically.
func TestCodeIndexScaling(t *testing.T) {
	makeFile := func(n int) *File {
		f := &File{}
		for i := range n {
			code := []byte{byte(i >> 16), byte(i >> 8), byte(i)}
			f.CIDSingles = append(f.CIDSingles, Single{Code: code, Value: cid.CID(i)})
			lo := []byte{byte(i >> 8), byte(i), 0x00}
			hi := []byte{byte(i >> 8), byte(i), byte(i % 200)}
			f.CIDRanges = append(f.CIDRanges, Range{First: lo, Last: hi, Value: cid.CID(i)})
		}
		// a range in which the first byte varies
		f.CIDRanges = append(f.CIDRanges, Range{First: []byte{0x00, 0x00}, Last: []byte{0xFF, 0xFF}})
		return f
	}

	// cost returns the allocations for building the index, and the
	// largest number of entries a lookup inspects.
	cost := func(n int) (float64, int) {
		f := makeFile(n)
		allocs := testing.AllocsPerRun(3, func() { buildFileLevel(f) })
		x := buildFileLevel(f).mappedIndex
		maxSegs := 0
		for _, n := range x.nodes {
			maxSegs = max(maxSegs, len(n.segs))
		}
		steps := len(x.leftover)
		for maxSegs > 0 {
			steps++
			maxSegs /= 2
		}
		return allocs, steps
	}

	a1, s1 := cost(20000)
	a2, s2 := cost(40000)
	t.Logf("n=20000: %g allocations, %d steps; n=40000: %g allocations, %d steps", a1, s1, a2, s2)
	if a2 > 2.5*a1 {
		t.Errorf("allocations grow too fast: %g for n=20000, %g for n=40000", a1, a2)
	}
	if s2 > s1+1 {
		t.Errorf("lookup steps grow too fast: %d for n=20000, %d for n=40000", s1, s2)
	}
}

// TestLookupConcurrent checks that concurrent first lookups are safe.
func TestLookupConcurrent(t *testing.T) {
	f := &File{CIDRanges: []Range{{First: []byte{0x00, 0x00}, Last: []byte{0x00, 0xFF}, Value: 1}}}
	done := make(chan cid.CID)
	for range 8 {
		go func() { done <- f.LookupCID([]byte{0x00, 0x05}) }()
	}
	for range 8 {
		if c := <-done; c != 6 {
			t.Errorf("CID %d, want 6", c)
		}
	}
}

// storeStream writes a stream with the given dictionary and body to a new
// in-memory PDF file, and returns the file together with the reference.
func storeStream(t *testing.T, dict pdf.Dict, body []byte) (pdf.Getter, pdf.Reference) {
	t.Helper()
	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
	ref := w.Alloc()
	stm, err := w.OpenStream(ref, dict)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stm.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := stm.Close(); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return w, ref
}

// hostileRanges returns n ranges of the shape <01010100> <FEFEFExx>, each
// of which gives the largest number of index pieces for four-byte codes.
func hostileRanges(n int) []Range {
	var res []Range
	for i := range n {
		res = append(res, Range{
			First: []byte{0x01, 0x01, 0x01, 0x00},
			Last:  []byte{0xFE, 0xFE, 0xFE, byte(0x7F - i%64)},
			Value: cid.CID(i),
		})
	}
	return res
}

// maxHostileRanges is the largest number of hostile ranges, see
// [hostileRanges], whose extra pieces fit into the budget of an index.
const maxHostileRanges = limits.MaxCMapIndexExtraPieces / (14*14*14 - 1)

// TestExtractTooComplex checks that an embedded CMap whose index would
// exceed the piece budget is rejected as malformed.
func TestExtractTooComplex(t *testing.T) {
	f := &File{
		Name:           "Evil",
		ROS:            &cid.SystemInfo{Registry: "A", Ordering: "B"},
		CodeSpaceRange: fullByteRange,
		CIDRanges:      hostileRanges(maxHostileRanges + 1),
	}
	body := &bytes.Buffer{}
	if err := cmapTmplNew.Execute(body, templateData{File: f}); err != nil {
		t.Fatal(err)
	}
	r, ref := storeStream(t, pdf.Dict{"Type": pdf.Name("CMap")}, body.Bytes())
	_, err := Extract(pdf.NewCursor(r), ref, false)
	if !pdf.IsMalformed(err) {
		t.Errorf("expected malformed file error, got %v", err)
	}

	// files built through the API are rejected when written
	if err := f.WriteTo(io.Discard, false); err == nil {
		t.Error("WriteTo accepted a CMap which is too complex")
	}
}

// TestExtractToUnicodeTooComplex checks that a ToUnicode CMap whose index
// would exceed the piece budget is dropped as if absent.
func TestExtractToUnicodeTooComplex(t *testing.T) {
	tu := &ToUnicodeFile{CodeSpaceRange: fullByteRange}
	for _, r := range hostileRanges(maxHostileRanges + 1) {
		tu.Ranges = append(tu.Ranges, ToUnicodeRange{First: r.First, Last: r.Last, Values: []string{"A"}})
	}
	body := &bytes.Buffer{}
	if err := toUnicodeTmplNew.Execute(body, tu); err != nil {
		t.Fatal(err)
	}
	r, ref := storeStream(t, pdf.Dict{}, body.Bytes())
	got, err := pdf.NewCursor(r).DecodeOptional(ref, ExtractToUnicode)
	if err != nil || got != nil {
		t.Errorf("got %v, %v; want nil, nil", got, err)
	}

	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
	rm := pdf.NewResourceManager(w)
	if _, err := rm.Embed(tu); err == nil {
		t.Error("Embed accepted a ToUnicode CMap which is too complex")
	}
}

// TestRectangularRange checks that a range in which more than the last byte
// varies maps the codes of the rectangle to consecutive CIDs.
func TestRectangularRange(t *testing.T) {
	f := &File{
		CodeSpaceRange: charcode.UCS2,
		CIDRanges:      []Range{{First: []byte{0x30, 0x30}, Last: []byte{0x32, 0x32}, Value: 34}},
	}
	want := cid.CID(34)
	for hi := byte(0x30); hi <= 0x32; hi++ {
		for lo := byte(0x30); lo <= 0x32; lo++ {
			if got := f.LookupCID([]byte{hi, lo}); got != want {
				t.Errorf("LookupCID(%02x%02x) = %d, want %d", hi, lo, got, want)
			}
			want++
		}
	}
	if got := f.LookupCID([]byte{0x30, 0x33}); got != 0 {
		t.Errorf("LookupCID(3033) = %d, want 0", got)
	}
}

// TestPredefinedWithinBound checks that the predefined CMaps are within the
// complexity bound.
func TestPredefinedWithinBound(t *testing.T) {
	for _, name := range allPredefined {
		f, err := Predefined(name)
		if err != nil {
			t.Fatal(err)
		}
		if f.index().tooComplex() {
			t.Errorf("%s: too complex", name)
		}
	}
}

// TestLookupCycle checks lookups in a chain whose Parent links end in a
// cycle, which a file cannot produce but a caller can construct.  Each CMap
// in the chain is consulted once, child first.
func TestLookupCycle(t *testing.T) {
	a := &File{CIDSingles: []Single{{Code: []byte{1}, Value: 1}}}
	b := &File{CIDSingles: []Single{{Code: []byte{1}, Value: 2}, {Code: []byte{2}, Value: 2}}}
	c := &File{
		CIDSingles:    []Single{{Code: []byte{3}, Value: 3}},
		NotdefSingles: []Single{{Code: []byte{4}, Value: 4}},
	}
	a.Parent = b
	b.Parent = c
	c.Parent = b

	// index b first, so that a finds a cached ancestor
	if got := b.LookupCID([]byte{1}); got != 2 {
		t.Errorf("b: CID %d, want 2", got)
	}
	// codes 0 to 5; code 4 has only a notdef mapping
	cases := []struct {
		f    *File
		want []cid.CID
	}{
		{a, []cid.CID{0, 1, 2, 3, 4, 0}},
		{b, []cid.CID{0, 2, 2, 3, 4, 0}},
		{c, []cid.CID{0, 2, 2, 3, 4, 0}},
	}
	for i, tc := range cases {
		for code, want := range tc.want {
			if got := tc.f.LookupCID([]byte{byte(code)}); got != want {
				t.Errorf("%d: code %d: CID %d, want %d", i, code, got, want)
			}
		}
		n := 0
		for code, cidVal := range tc.f.All(charcode.SimpleCodec) {
			n++
			if cidVal != tc.want[code] {
				t.Errorf("%d: All: code %d gives CID %d, want %d", i, code, cidVal, tc.want[code])
			}
		}
		if n != 3 {
			t.Errorf("%d: All gave %d codes, want 3", i, n)
		}
	}

	// a cycle with no cached member
	d := &File{CIDSingles: []Single{{Code: []byte{5}, Value: 5}}}
	e := &File{CIDSingles: []Single{{Code: []byte{5}, Value: 6}, {Code: []byte{6}, Value: 6}}}
	d.Parent = e
	e.Parent = d
	if got := e.LookupCID([]byte{6}); got != 6 {
		t.Errorf("e: CID %d, want 6", got)
	}
	if got := d.LookupCID([]byte{6}); got != 6 {
		t.Errorf("d: CID %d, want 6", got)
	}
	if got := d.LookupCID([]byte{5}); got != 5 {
		t.Errorf("d: CID %d, want 5", got)
	}
}

// TestChainIndexScaling checks that indexing every level of a usecmap chain,
// as reading the chain from a file does, and enumerating the mappings of
// the chain take memory linear in the total size of the chain.
func TestChainIndexScaling(t *testing.T) {
	const m = 200 // entries per level

	makeFiles := func(depth int) []*File {
		var res []*File
		var parent *File
		for l := range depth {
			f := &File{Parent: parent}
			for i := range m {
				code := []byte{byte(l), byte(i), 0}
				f.CIDSingles = append(f.CIDSingles, Single{Code: code, Value: cid.CID(i)})
			}
			res = append(res, f)
			parent = f
		}
		return res
	}
	makeToUnicode := func(depth int) []*ToUnicodeFile {
		var res []*ToUnicodeFile
		var parent *ToUnicodeFile
		for l := range depth {
			tu := &ToUnicodeFile{Parent: parent}
			for i := range m {
				code := []byte{byte(l), byte(i), 0}
				tu.Singles = append(tu.Singles, ToUnicodeSingle{Code: code, Value: "A"})
			}
			res = append(res, tu)
			parent = tu
		}
		return res
	}
	codec, err := charcode.NewCodec(charcode.CodeSpaceRange{
		{Low: []byte{0x00, 0x00, 0x00}, High: []byte{0xFF, 0xFF, 0xFF}},
	})
	if err != nil {
		t.Fatal(err)
	}

	// size returns the number of index segments held for the chain, and
	// the allocations for indexing it level by level, root first, and
	// enumerating its mappings.
	size := func(depth int) (int, float64) {
		segs := 0
		for _, f := range makeFiles(depth) {
			for _, n := range f.index().level.mappedIndex.nodes {
				segs += len(n.segs)
			}
		}
		for _, tu := range makeToUnicode(depth) {
			for _, n := range tu.index().level.index.nodes {
				segs += len(n.segs)
			}
		}

		allocs := testing.AllocsPerRun(2, func() {
			files := makeFiles(depth)
			for _, f := range files {
				f.LookupCID([]byte{0, 0, 0})
			}
			for range files[len(files)-1].All(codec) {
			}
			tus := makeToUnicode(depth)
			for _, tu := range tus {
				tu.Lookup([]byte{0, 0, 0})
			}
			tus[len(tus)-1].CodeForText("B")
		})
		return segs, allocs
	}

	s1, a1 := size(16)
	s2, a2 := size(32)
	t.Logf("depth 16: %d segments, %g allocations; depth 32: %d segments, %g allocations", s1, a1, s2, a2)
	if s2 > 2*s1 {
		t.Errorf("index size grows too fast: %d for depth 16, %d for depth 32", s1, s2)
	}
	if a2 > 2.5*a1 {
		t.Errorf("allocations grow too fast: %g for depth 16, %g for depth 32", a1, a2)
	}
}

// longCodeCMap is a CMap in which every kind of entry also occurs with
// codes longer than [maxCodeLength] bytes.
const longCodeCMap = `/CIDInit /ProcSet findresource begin
12 dict begin
begincmap
/CIDSystemInfo 3 dict dup begin
/Registry (A) def
/Ordering (B) def
/Supplement 0 def
end def
/CMapName /Long def
/CMapType 1 def
2 begincodespacerange
<00> <ff>
<0000000000> <ffffffffff>
endcodespacerange
2 begincidchar
<41> 1
<0000000041> 2
endcidchar
2 begincidrange
<50> <5f> 10
<0000000050> <000000005f> 20
endcidrange
2 beginnotdefchar
<01> 3
<0000000001> 4
endnotdefchar
2 beginnotdefrange
<02> <0f> 5
<0000000002> <000000000f> 6
endnotdefrange
endcmap
CMapName currentdict /CMap defineresource pop
end
end
`

// TestReadCMapLongCodes checks that entries with codes longer than
// [maxCodeLength] bytes are dropped when a CMap is read.
func TestReadCMapLongCodes(t *testing.T) {
	f, _, err := readCMap(strings.NewReader(longCodeCMap))
	if err != nil {
		t.Fatal(err)
	}
	want := &File{
		Name:           "Long",
		ROS:            &cid.SystemInfo{Registry: "A", Ordering: "B"},
		CodeSpaceRange: charcode.CodeSpaceRange{{Low: []byte{0x00}, High: []byte{0xFF}}},
		CIDSingles:     []Single{{Code: []byte{0x41}, Value: 1}},
		CIDRanges:      []Range{{First: []byte{0x50}, Last: []byte{0x5F}, Value: 10}},
		NotdefSingles:  []Single{{Code: []byte{0x01}, Value: 3}},
		NotdefRanges:   []Range{{First: []byte{0x02}, Last: []byte{0x0F}, Value: 5}},
	}
	if diff := cmp.Diff(want, f); diff != "" {
		t.Errorf("wrong CMap (-want +got):\n%s", diff)
	}

	buf := &bytes.Buffer{}
	if err := f.WriteTo(buf, false); err != nil {
		t.Fatal(err)
	}
	f2, _, err := readCMap(buf)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(f, f2); diff != "" {
		t.Errorf("round trip failed (-want +got):\n%s", diff)
	}
}

// longToUnicodeCMap is a ToUnicode CMap in which every kind of entry also
// occurs with codes longer than [maxCodeLength] bytes.
const longToUnicodeCMap = `/CIDInit /ProcSet findresource begin
12 dict begin
begincmap
/CMapName /Long def
/CMapType 2 def
2 begincodespacerange
<00> <ff>
<0000000000> <ffffffffff>
endcodespacerange
2 beginbfchar
<41> <0041>
<0000000041> <0042>
endbfchar
2 beginbfrange
<50> <5f> <0061>
<0000000050> <000000005f> <0071>
endbfrange
endcmap
CMapName currentdict /CMap defineresource pop
end
end
`

// TestReadToUnicodeLongCodes checks that entries with codes longer than
// [maxCodeLength] bytes are dropped when a ToUnicode CMap is read.
func TestReadToUnicodeLongCodes(t *testing.T) {
	tu, err := readToUnicode(strings.NewReader(longToUnicodeCMap))
	if err != nil {
		t.Fatal(err)
	}
	want := &ToUnicodeFile{
		CodeSpaceRange: charcode.CodeSpaceRange{{Low: []byte{0x00}, High: []byte{0xFF}}},
		Singles:        []ToUnicodeSingle{{Code: []byte{0x41}, Value: "A"}},
		Ranges:         []ToUnicodeRange{{First: []byte{0x50}, Last: []byte{0x5F}, Values: []string{"a"}}},
	}
	if diff := cmp.Diff(want, tu); diff != "" {
		t.Errorf("wrong CMap (-want +got):\n%s", diff)
	}

	buf := &bytes.Buffer{}
	if err := toUnicodeTmplNew.Execute(buf, tu); err != nil {
		t.Fatal(err)
	}
	tu2, err := readToUnicode(buf)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(tu, tu2); diff != "" {
		t.Errorf("round trip failed (-want +got):\n%s", diff)
	}
}

// longRangesBody returns the body of a CMap with a single mapping and n
// ranges of six-byte codes.  Unless dropped, the long ranges would make the
// CMap too complex.
func longRangesBody(n int, toUnicode bool) []byte {
	b := &bytes.Buffer{}
	b.WriteString("/CIDInit /ProcSet findresource begin\n12 dict begin\nbegincmap\n")
	if toUnicode {
		b.WriteString("/CMapName /Long def\n/CMapType 2 def\n")
	} else {
		b.WriteString("/CIDSystemInfo 3 dict dup begin\n/Registry (A) def\n/Ordering (B) def\n/Supplement 0 def\nend def\n")
		b.WriteString("/CMapName /Long def\n/CMapType 1 def\n")
	}
	b.WriteString("1 begincodespacerange\n<00> <ff>\nendcodespacerange\n")
	if toUnicode {
		b.WriteString("1 beginbfchar\n<41> <0041>\nendbfchar\n")
		fmt.Fprintf(b, "%d beginbfrange\n", n)
		for i := range n {
			fmt.Fprintf(b, "<000000000000> <ffffffffff%02x> <0041>\n", i)
		}
		b.WriteString("endbfrange\n")
	} else {
		b.WriteString("1 begincidchar\n<41> 7\nendcidchar\n")
		fmt.Fprintf(b, "%d begincidrange\n", n)
		for i := range n {
			fmt.Fprintf(b, "<000000000000> <ffffffffff%02x> 1\n", i)
		}
		b.WriteString("endcidrange\n")
	}
	b.WriteString("endcmap\nCMapName currentdict /CMap defineresource pop\nend\nend\n")
	return b.Bytes()
}

// TestExtractLongRanges checks that ranges of codes longer than
// [maxCodeLength] bytes do not make a CMap unreadable.
func TestExtractLongRanges(t *testing.T) {
	r, ref := storeStream(t, pdf.Dict{"Type": pdf.Name("CMap")}, longRangesBody(30, false))
	f, err := Extract(pdf.NewCursor(r), ref, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.CIDRanges) != 0 {
		t.Errorf("%d long ranges kept", len(f.CIDRanges))
	}
	if got := f.LookupCID([]byte{0x41}); got != 7 {
		t.Errorf("CID %d, want 7", got)
	}

	r, ref = storeStream(t, pdf.Dict{}, longRangesBody(30, true))
	tu, err := ExtractToUnicode(pdf.NewCursor(r), ref, false)
	if err != nil {
		t.Fatal(err)
	}
	if tu == nil {
		t.Fatal("ToUnicode CMap dropped")
	}
	if len(tu.Ranges) != 0 {
		t.Errorf("%d long ranges kept", len(tu.Ranges))
	}
	if got, ok := tu.Lookup([]byte{0x41}); !ok || got != "A" {
		t.Errorf("got %q, %t; want \"A\", true", got, ok)
	}
}

// apiChain returns the top of a usecmap chain of n CMaps, built through the
// API.  Every CMap maps the code of its own level.
func apiChain(n int) *File {
	var f *File
	for l := range n {
		f = &File{
			Name:           fmt.Sprintf("Level%d", l),
			ROS:            &cid.SystemInfo{Registry: "A", Ordering: "B"},
			CodeSpaceRange: charcode.Simple,
			CIDSingles:     []Single{{Code: []byte{byte(l)}, Value: cid.CID(l + 1)}},
			Parent:         f,
		}
	}
	return f
}

// apiToUnicodeChain returns the top of a usecmap chain of n ToUnicode
// CMaps, built through the API.
func apiToUnicodeChain(n int) *ToUnicodeFile {
	var tu *ToUnicodeFile
	for l := range n {
		tu = &ToUnicodeFile{
			CodeSpaceRange: charcode.Simple,
			Singles:        []ToUnicodeSingle{{Code: []byte{byte(l)}, Value: string(rune('A' + l))}},
			Parent:         tu,
		}
	}
	return tu
}

// writeRawChain writes the chain starting at f to w, one stream per CMap,
// without the checks of [File.Embed].  It returns the reference of f.
func writeRawChain(t *testing.T, w *pdf.Writer, f *File) pdf.Reference {
	t.Helper()
	var parent pdf.Object
	if f.Parent != nil {
		parent = writeRawChain(t, w, f.Parent)
	}
	body := &bytes.Buffer{}
	level := f.Clone()
	level.Parent = nil
	if err := cmapTmplNew.Execute(body, templateData{File: level}); err != nil {
		t.Fatal(err)
	}
	dict := pdf.Dict{"Type": pdf.Name("CMap"), "CMapName": pdf.Name(f.Name)}
	if parent != nil {
		dict["UseCMap"] = parent
	}
	return writeRawStream(t, w, dict, body.Bytes())
}

// writeRawToUnicodeChain writes the chain starting at tu to w, one stream
// per CMap, without the checks of [ToUnicodeFile.Embed].
func writeRawToUnicodeChain(t *testing.T, w *pdf.Writer, tu *ToUnicodeFile) pdf.Reference {
	t.Helper()
	var parent pdf.Object
	if tu.Parent != nil {
		parent = writeRawToUnicodeChain(t, w, tu.Parent)
	}
	body := &bytes.Buffer{}
	level := *tu
	level.Parent = nil
	if err := toUnicodeTmplNew.Execute(body, &level); err != nil {
		t.Fatal(err)
	}
	dict := pdf.Dict{}
	if parent != nil {
		dict["UseCMap"] = parent
	}
	return writeRawStream(t, w, dict, body.Bytes())
}

func writeRawStream(t *testing.T, w *pdf.Writer, dict pdf.Dict, body []byte) pdf.Reference {
	t.Helper()
	ref := w.Alloc()
	stm, err := w.OpenStream(ref, dict)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stm.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := stm.Close(); err != nil {
		t.Fatal(err)
	}
	return ref
}

// TestChainDepthLimit checks that a usecmap chain of
// [limits.MaxCMapChainDepth] CMaps is read and written, and that a longer
// chain is rejected by both.
func TestChainDepthLimit(t *testing.T) {
	n := limits.MaxCMapChainDepth

	// a chain at the limit reads and round-trips
	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
	ref := writeRawChain(t, w, apiChain(n))
	f1, err := pdf.NewCursor(w).Decode(ref, Extract)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(f1.Chain()); got != n {
		t.Fatalf("chain of %d CMaps, want %d", got, n)
	}
	rm := pdf.NewResourceManager(w)
	ref2, err := rm.Embed(f1)
	if err != nil {
		t.Fatal(err)
	}
	f2, err := pdf.NewCursor(w).Decode(ref2, Extract)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(f1, f2); diff != "" {
		t.Errorf("round trip failed (-want +got):\n%s", diff)
	}

	// a longer chain is rejected on read
	ref = writeRawChain(t, w, apiChain(n+1))
	if _, err := pdf.NewCursor(w).Decode(ref, Extract); !pdf.IsMalformed(err) {
		t.Errorf("expected malformed file error, got %v", err)
	}

	// a longer chain is rejected on write, before anything is written
	before := w.Alloc()
	if _, err := rm.Embed(apiChain(n + 1)); err == nil {
		t.Error("Embed accepted a chain which is too deep")
	}
	if after := w.Alloc(); after.Number() != before.Number()+1 {
		t.Errorf("Embed allocated %d references", after.Number()-before.Number()-1)
	}
	if err := apiChain(n+1).WriteTo(io.Discard, false); err == nil {
		t.Error("WriteTo accepted a chain which is too deep")
	}

	// lookups in chains built through the API are not limited
	if got := apiChain(n + 1).LookupCID([]byte{0}); got != 1 {
		t.Errorf("CID %d, want 1", got)
	}
}

// TestToUnicodeChainDepthLimit checks the chain depth limit for ToUnicode
// CMaps.
func TestToUnicodeChainDepthLimit(t *testing.T) {
	n := limits.MaxCMapChainDepth

	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
	ref := writeRawToUnicodeChain(t, w, apiToUnicodeChain(n))
	tu1, err := pdf.NewCursor(w).Decode(ref, ExtractToUnicode)
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := tu1.Lookup([]byte{0}); !ok || got != "A" {
		t.Errorf("got %q, %t; want \"A\", true", got, ok)
	}
	rm := pdf.NewResourceManager(w)
	ref2, err := rm.Embed(tu1)
	if err != nil {
		t.Fatal(err)
	}
	tu2, err := pdf.NewCursor(w).Decode(ref2, ExtractToUnicode)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(tu1, tu2); diff != "" {
		t.Errorf("round trip failed (-want +got):\n%s", diff)
	}

	ref = writeRawToUnicodeChain(t, w, apiToUnicodeChain(n+1))
	if _, err := pdf.NewCursor(w).Decode(ref, ExtractToUnicode); !pdf.IsMalformed(err) {
		t.Errorf("expected malformed file error, got %v", err)
	}

	before := w.Alloc()
	if _, err := rm.Embed(apiToUnicodeChain(n + 1)); err == nil {
		t.Error("Embed accepted a chain which is too deep")
	}
	if after := w.Alloc(); after.Number() != before.Number()+1 {
		t.Errorf("Embed allocated %d references", after.Number()-before.Number()-1)
	}
}

// TestWriteCycle checks that a chain whose Parent links form a cycle is
// rejected on write.
func TestWriteCycle(t *testing.T) {
	a := apiChain(1)
	b := apiChain(1)
	b.Name = "Other"
	a.Parent = b
	b.Parent = a
	if err := a.WriteTo(io.Discard, false); err == nil {
		t.Error("WriteTo accepted a cyclic chain")
	}
	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
	rm := pdf.NewResourceManager(w)
	if _, err := rm.Embed(a); err == nil {
		t.Error("Embed accepted a cyclic chain")
	}

	tuA := apiToUnicodeChain(1)
	tuB := apiToUnicodeChain(1)
	tuA.Parent = tuB
	tuB.Parent = tuA
	if _, err := rm.Embed(tuA); err == nil {
		t.Error("Embed accepted a cyclic ToUnicode chain")
	}
}

// TestPredefinedChainDepth checks that the predefined CMaps are within the
// chain depth limit.
func TestPredefinedChainDepth(t *testing.T) {
	for _, name := range allPredefined {
		f, err := Predefined(name)
		if err != nil {
			t.Fatal(err)
		}
		if d := f.index().depth; d > limits.MaxCMapChainDepth {
			t.Errorf("%s: chain of %d CMaps", name, d)
		}
	}
}

// TestCanonicalNodes checks that the canonical nodes of every byte interval
// cover the interval exactly once, and checks their number for some
// intervals.
func TestCanonicalNodes(t *testing.T) {
	for lo := range 256 {
		for hi := lo; hi < 256; hi++ {
			nodes := canonicalNodes(nil, byte(lo), byte(hi))
			if len(nodes) > 14 {
				t.Fatalf("%02x-%02x: %d nodes", lo, hi, len(nodes))
			}
			var count [256]int
			for _, nd := range nodes {
				s := 8 - int(nd.depth)
				for v := int(nd.index) << s; v < (int(nd.index)+1)<<s; v++ {
					count[v]++
				}
			}
			for v := range 256 {
				want := 0
				if v >= lo && v <= hi {
					want = 1
				}
				if count[v] != want {
					t.Fatalf("%02x-%02x: value %02x covered %d times", lo, hi, v, count[v])
				}
			}
		}
	}

	cases := []struct {
		lo, hi byte
		want   int
	}{
		{0x41, 0x41, 1},
		{0x00, 0xFF, 1},
		{0x20, 0x33, 2},
		{0x81, 0xFE, 12},
		{0x01, 0xFE, 14},
	}
	for _, c := range cases {
		if got := len(canonicalNodes(nil, c.lo, c.hi)); got != c.want {
			t.Errorf("%02x-%02x: %d nodes, want %d", c.lo, c.hi, got, c.want)
		}
	}
	if got := extraPieces([]byte{0x01, 0x01, 0x01, 0x00}, []byte{0xFE, 0xFE, 0xFE, 0x7F}); got != 14*14*14-1 {
		t.Errorf("four-byte box: %d extra pieces, want %d", got, 14*14*14-1)
	}
	if got := extraPieces([]byte{0x81, 0x30, 0x81, 0x30}, []byte{0xFE, 0x39, 0xFE, 0x39}); got != 287 {
		t.Errorf("four-byte box: %d extra pieces, want 287", got)
	}
}

// TestCommonCasePieces checks that ranges in which only the last byte
// varies give one piece each, and that lookups in their index follow the
// code directly.
func TestCommonCasePieces(t *testing.T) {
	const n = 3000
	var boxes []box
	pieces := 0
	for i := range n {
		first := []byte{byte(i >> 4), 0x10, byte(i%16) * 16}
		last := []byte{byte(i >> 4), 0x10, byte(i%16)*16 + 7}
		boxes = append(boxes, box{first, last, i})
		pieces += 1 + extraPieces(first, last)
	}
	if pieces != n {
		t.Errorf("%d pieces, want %d", pieces, n)
	}
	x := newCodeIndex(boxes)
	segs := 0
	for _, nd := range x.nodes {
		segs += len(nd.segs)
	}
	if segs != n {
		t.Errorf("%d segments, want %d", segs, n)
	}
	if x.depths != 1<<8 {
		t.Errorf("occupied depths %09b, want only depth 8", x.depths)
	}
	for i, b := range boxes {
		if got := x.lookup(b.last); got != i {
			t.Fatalf("% x: id %d, want %d", b.last, got, i)
		}
	}
}

// bruteLookup returns the smallest id of the boxes which contain code, or
// -1.
func bruteLookup(boxes []box, code []byte) int {
	best := -1
	for _, b := range boxes {
		if boxContains(b.first, b.last, code) && (best < 0 || b.id < best) {
			best = b.id
		}
	}
	return best
}

// randomInterval returns a byte interval which is constant, full, aligned
// to a segment-tree node, or arbitrary.
func randomInterval(rng *rand.Rand) (byte, byte) {
	switch rng.IntN(4) {
	case 0:
		v := byte(rng.IntN(256))
		return v, v
	case 1:
		return 0x00, 0xFF
	case 2:
		s := rng.IntN(9)
		k := rng.IntN(256 >> s)
		return byte(k << s), byte((k+1)<<s - 1)
	default:
		a, b := byte(rng.IntN(256)), byte(rng.IntN(256))
		return min(a, b), max(a, b)
	}
}

// TestCodeIndexBruteForce compares lookups in a [codeIndex] with a scan of
// all boxes.
func TestCodeIndexBruteForce(t *testing.T) {
	rng := rand.New(rand.NewPCG(13, 14))
	for range 300 {
		var boxes []box
		for id := range 1 + rng.IntN(40) {
			l := 1 + rng.IntN(4)
			first, last := make([]byte, l), make([]byte, l)
			for i := range l {
				first[i], last[i] = randomInterval(rng)
			}
			boxes = append(boxes, box{first, last, id})
		}
		rng.Shuffle(len(boxes), func(i, j int) { boxes[i], boxes[j] = boxes[j], boxes[i] })
		x := newCodeIndex(boxes)
		if len(x.leftover) > 0 {
			t.Fatal("unexpected leftover boxes")
		}

		for range 300 {
			b := boxes[rng.IntN(len(boxes))]
			code := make([]byte, len(b.first))
			for i := range code {
				switch rng.IntN(4) {
				case 0:
					code[i] = b.first[i]
				case 1:
					code[i] = b.last[i]
				case 2:
					// just outside, if possible
					if rng.IntN(2) == 0 {
						code[i] = b.first[i] - 1
					} else {
						code[i] = b.last[i] + 1
					}
				default:
					code[i] = byte(rng.IntN(256))
				}
			}
			if got, want := x.lookup(code), bruteLookup(boxes, code); got != want {
				t.Fatalf("% x: id %d, want %d", code, got, want)
			}
		}
	}
}

// awkwardTwoByteRanges returns n two-byte ranges with first byte 01 to FE,
// each of which gives 14 index pieces.
func awkwardTwoByteRanges(n int) []Range {
	var res []Range
	for i := range n {
		res = append(res, Range{
			First: []byte{0x01, byte(i)},
			Last:  []byte{0xFE, byte(i)},
			Value: cid.CID(i),
		})
	}
	return res
}

// maxTwoByteRanges is the largest number of ranges, see
// [awkwardTwoByteRanges], whose extra pieces fit into the budget.
const maxTwoByteRanges = limits.MaxCMapIndexExtraPieces / 13

// TestTwoByteRanges checks that a thousand two-byte ranges in which the
// first byte varies fit into an index, and checks the boundary of the
// piece budget.
func TestTwoByteRanges(t *testing.T) {
	f := &File{CodeSpaceRange: charcode.UCS2, CIDRanges: awkwardTwoByteRanges(maxTwoByteRanges)}
	if f.index().tooComplex() {
		t.Error("two-byte CMap is too complex")
	}
	for _, code := range [][]byte{{0x80, 0x05}, {0x01, 0x00}, {0xFE, 0xFF}, {0x00, 0x05}, {0xFF, 0x05}} {
		if got, want := f.LookupCID(code), refLookupCID(f, code); got != want {
			t.Errorf("% x: CID %d, want %d", code, got, want)
		}
	}

	if fileLevelFits(&File{CIDRanges: awkwardTwoByteRanges(maxTwoByteRanges + 1)}) {
		t.Errorf("%d ranges fit", maxTwoByteRanges+1)
	}
	if maxTwoByteRanges != 1260 || maxHostileRanges != 5 {
		t.Errorf("%d two-byte, %d four-byte ranges fit; want 1260, 5", maxTwoByteRanges, maxHostileRanges)
	}
	gb := Range{First: []byte{0x81, 0x30, 0x81, 0x30}, Last: []byte{0xFE, 0x39, 0xFE, 0x39}}
	n := limits.MaxCMapIndexExtraPieces / 287
	if n != 57 {
		t.Errorf("%d ranges like <81308130> <FE39FE39> fit, want 57", n)
	}
	if !fileLevelFits(&File{CIDRanges: slices.Repeat([]Range{gb}, n)}) {
		t.Errorf("%d ranges like <81308130> <FE39FE39> do not fit", n)
	}
	if fileLevelFits(&File{CIDRanges: slices.Repeat([]Range{gb}, n+1)}) {
		t.Errorf("%d ranges like <81308130> <FE39FE39> fit", n+1)
	}
}

// TestPiecesFitAgrees checks that [piecesFit] reports leftover boxes
// exactly when [newCodeIndex] leaves boxes over, for random box sets
// around the budget.
func TestPiecesFitAgrees(t *testing.T) {
	rng := rand.New(rand.NewPCG(17, 18))
	var seen [2]bool
	for range 60 {
		var boxes []box
		total := 0
		target := limits.MaxCMapIndexExtraPieces/2 + rng.IntN(limits.MaxCMapIndexExtraPieces)
		for total < target {
			l := rng.IntN(5)
			first, last := make([]byte, l), make([]byte, l)
			for i := range l {
				first[i], last[i] = randomInterval(rng)
			}
			if rng.IntN(20) == 0 && l > 0 {
				// no codes
				first[0], last[0] = 0x10, 0x0F
			}
			bx := box{first, last, len(boxes)}
			if boxIsValid(first, last) && l > 0 {
				total += extraPieces(first, last)
			}
			boxes = append(boxes, bx)
		}
		x := newCodeIndex(boxes)
		if fits, built := piecesFit(boxes), len(x.leftover) == 0; fits != built {
			t.Fatalf("%d pieces: piecesFit %t, index without leftover %t", total, fits, built)
		} else if fits {
			seen[0] = true
		} else {
			seen[1] = true
		}
	}
	if !seen[0] || !seen[1] {
		t.Error("box sets did not reach both sides of the budget")
	}
}

// TestLeadingByteRanges checks that a CMap of 200 ranges <20xx> <33xx> is
// written and read, and maps the codes in and next to the ranges.
func TestLeadingByteRanges(t *testing.T) {
	f := &File{
		Name:           "Lead",
		ROS:            &cid.SystemInfo{Registry: "A", Ordering: "B"},
		CodeSpaceRange: charcode.UCS2,
	}
	tu := &ToUnicodeFile{CodeSpaceRange: charcode.UCS2}
	for i := range 200 {
		first, last := []byte{0x20, byte(i)}, []byte{0x33, byte(i)}
		f.CIDRanges = append(f.CIDRanges, Range{First: first, Last: last, Value: cid.CID(100 + 20*i)})
		tu.Ranges = append(tu.Ranges, ToUnicodeRange{First: first, Last: last, Values: []string{"A"}})
	}

	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
	rm := pdf.NewResourceManager(w)
	ref, err := rm.Embed(f)
	if err != nil {
		t.Fatal(err)
	}
	f1, err := pdf.NewCursor(w).Decode(ref, Extract)
	if err != nil {
		t.Fatal(err)
	}
	ref, err = rm.Embed(f1)
	if err != nil {
		t.Fatal(err)
	}
	f2, err := pdf.NewCursor(w).Decode(ref, Extract)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(f1, f2); diff != "" {
		t.Errorf("round trip failed (-want +got):\n%s", diff)
	}

	tuRef, err := rm.Embed(tu)
	if err != nil {
		t.Fatal(err)
	}
	tu1, err := pdf.NewCursor(w).Decode(tuRef, ExtractToUnicode)
	if err != nil {
		t.Fatal(err)
	}
	tuRef, err = rm.Embed(tu1)
	if err != nil {
		t.Fatal(err)
	}
	tu2, err := pdf.NewCursor(w).Decode(tuRef, ExtractToUnicode)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(tu1, tu2); diff != "" {
		t.Errorf("round trip failed (-want +got):\n%s", diff)
	}

	for _, i := range []int{0, 1, 99, 199} {
		cases := []struct {
			code []byte
			cid  cid.CID
			text string
		}{
			{[]byte{0x20, byte(i)}, cid.CID(100 + 20*i), "A"},
			{[]byte{0x2A, byte(i)}, cid.CID(100 + 20*i + 10), "K"},
			{[]byte{0x33, byte(i)}, cid.CID(100 + 20*i + 19), "T"},
			{[]byte{0x1F, byte(i)}, 0, ""},
			{[]byte{0x34, byte(i)}, 0, ""},
		}
		for _, c := range cases {
			if got := f1.LookupCID(c.code); got != c.cid {
				t.Errorf("% x: CID %d, want %d", c.code, got, c.cid)
			}
			got, ok := tu1.Lookup(c.code)
			if got != c.text || ok != (c.text != "") {
				t.Errorf("% x: text %q, %t, want %q", c.code, got, ok, c.text)
			}
		}
	}
	if got := f1.LookupCID([]byte{0x20, 200}); got != 0 {
		t.Errorf("CID %d, want 0", got)
	}
}

// TestPieceBudget checks that a CMap whose index just fits the piece budget
// is written and read, and that lookups in a CMap built through the API
// which exceeds the budget are correct.
func TestPieceBudget(t *testing.T) {
	f := &File{
		Name:           "Budget",
		ROS:            &cid.SystemInfo{Registry: "A", Ordering: "B"},
		CodeSpaceRange: fullByteRange,
		CIDRanges:      hostileRanges(maxHostileRanges),
	}
	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
	ref, err := pdf.NewResourceManager(w).Embed(f)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pdf.NewCursor(w).Decode(ref, Extract); err != nil {
		t.Fatal(err)
	}

	g := &File{CIDRanges: hostileRanges(maxHostileRanges + 1)}
	x := g.index()
	if !x.tooComplex() || len(x.level.mappedIndex.leftover) != 1 {
		t.Fatal("expected one leftover box")
	}
	rng := rand.New(rand.NewPCG(15, 16))
	for range 2000 {
		code := []byte{byte(rng.IntN(256)), byte(rng.IntN(256)), byte(rng.IntN(256)), byte(rng.IntN(256))}
		if rng.IntN(2) == 0 {
			code[3] = 0x7F - byte(rng.IntN(8))
		}
		if got, want := g.LookupCID(code), refLookupCID(g, code); got != want {
			t.Fatalf("% x: CID %d, want %d", code, got, want)
		}
	}
}

// TestLargeCommonCase checks that a large CMap whose entries have constant
// leading bytes fits into the index, is written and read, and maps its
// codes.
func TestLargeCommonCase(t *testing.T) {
	const n = 50000
	f := &File{
		Name:           "Large",
		ROS:            &cid.SystemInfo{Registry: "A", Ordering: "B"},
		CodeSpaceRange: fullByteRange,
	}
	for i := range n {
		f.CIDSingles = append(f.CIDSingles, Single{
			Code:  []byte{0x01, byte(i >> 16), byte(i >> 8), byte(i)},
			Value: cid.CID(i),
		})
		f.CIDRanges = append(f.CIDRanges, Range{
			First: []byte{0x02, byte(i >> 8), byte(i), 0x00},
			Last:  []byte{0x02, byte(i >> 8), byte(i), 0x0F},
			Value: cid.CID(16 * i),
		})
	}

	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
	ref, err := pdf.NewResourceManager(w).Embed(f)
	if err != nil {
		t.Fatal(err)
	}
	g, err := pdf.NewCursor(w).Decode(ref, Extract)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(f, g); diff != "" {
		t.Errorf("round trip failed (-want +got):\n%s", diff)
	}

	rng := rand.New(rand.NewPCG(19, 20))
	for range 2000 {
		i := rng.IntN(n)
		codes := [][]byte{
			{0x01, byte(i >> 16), byte(i >> 8), byte(i)},
			{0x02, byte(i >> 8), byte(i), byte(rng.IntN(32))},
		}
		for _, code := range codes {
			if got, want := g.LookupCID(code), refLookupCID(f, code); got != want {
				t.Fatalf("% x: CID %d, want %d", code, got, want)
			}
		}
	}
}
