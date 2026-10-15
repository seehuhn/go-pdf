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

package ccittfax

import (
	"bytes"
	"fmt"
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/image/ccitt"
)

// testImage is a 16x6 bitmap with 1 = white.
var testImage = []byte{
	0xF0, 0xFF,
	0xF0, 0xFF,
	0x0F, 0xF0,
	0xFF, 0x00,
	0xAA, 0x55,
	0x00, 0x01,
}

func encode(t testing.TB, p *Params, image []byte) []byte {
	t.Helper()
	buf := &bytes.Buffer{}
	w, err := NewWriter(buf, p)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(image); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func decode(t testing.TB, p *Params, data []byte) ([]byte, error) {
	t.Helper()
	r, err := NewReader(bytes.NewReader(data), p)
	if err != nil {
		t.Fatal(err)
	}
	return io.ReadAll(r)
}

// TestRoundTripParams checks that every combination of the stream
// parameters survives a round trip through the writer and reader, with
// the reader's row count unbounded so that end-of-block detection is
// exercised.
func TestRoundTripParams(t *testing.T) {
	for _, k := range []int{-1, 0, 2} {
		for _, eol := range []bool{false, true} {
			for _, align := range []bool{false, true} {
				for _, ignoreEOB := range []bool{false, true} {
					name := fmt.Sprintf("K%d-eol%t-align%t-ignore%t", k, eol, align, ignoreEOB)
					t.Run(name, func(t *testing.T) {
						p := &Params{
							Columns:          16,
							K:                k,
							EndOfLine:        eol,
							EncodedByteAlign: align,
							IgnoreEndOfBlock: ignoreEOB,
						}
						data := encode(t, p, testImage)
						got, err := decode(t, p, data)
						if err != nil {
							t.Fatal(err)
						}
						if diff := cmp.Diff(testImage, got); diff != "" {
							t.Errorf("round trip failed (-want +got):\n%s", diff)
						}
					})
				}
			}
		}
	}
}

// TestAlignedCompatibility checks byte-aligned Group 4 output against the
// golang.org/x/image/ccitt decoder.  (That decoder's Group 3 variant
// requires EOL codes without fill bits, so it cannot read aligned
// Group 3 data.)
func TestAlignedCompatibility(t *testing.T) {
	p := &Params{
		Columns:          16,
		K:                -1,
		EncodedByteAlign: true,
	}
	data := encode(t, p, testImage)

	r := ccitt.NewReader(bytes.NewReader(data), ccitt.MSB, ccitt.Group4,
		p.Columns, len(testImage)/2, &ccitt.Options{Align: true})
	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(testImage, got); diff != "" {
		t.Errorf("x/image/ccitt disagrees (-want +got):\n%s", diff)
	}
}

// bitBuf assembles hand-written bit strings.
type bitBuf struct {
	data []byte
	n    int
}

func (b *bitBuf) write(bits string) {
	for _, c := range bits {
		switch c {
		case '0', '1':
			if b.n%8 == 0 {
				b.data = append(b.data, 0)
			}
			if c == '1' {
				b.data[b.n/8] |= 0x80 >> (b.n % 8)
			}
			b.n++
		}
	}
}

const (
	eol    = "000000000001"
	rowF0  = "1011 011"          // WWWWBBBB: white 4, black 4
	row0F  = "00110101 011 1011" // BBBBWWWW: white 0, black 4, white 4
	broken = "0000000011111"     // no such code
)

// TestDamagedRows checks that damaged rows are substituted and decoding
// resumes at the next EOL, up to DamagedRowsBeforeError rows.
func TestDamagedRows(t *testing.T) {
	var b bitBuf
	b.write(eol + rowF0)
	b.write(eol + broken)
	b.write(eol + row0F)
	b.write(eol + broken)
	b.write(eol + broken)
	b.write(eol + rowF0)
	for range 6 {
		b.write(eol)
	}

	cases := []struct {
		tolerate int
		want     []byte
		wantErr  bool
	}{
		{0, []byte{0xF0}, true},
		{1, []byte{0xF0, 0xF0, 0x0F}, true},
		{2, []byte{0xF0, 0xF0, 0x0F, 0x0F}, true},
		// the third damaged row follows a damaged row and becomes white
		{3, []byte{0xF0, 0xF0, 0x0F, 0x0F, 0xFF, 0xF0}, false},
		{9, []byte{0xF0, 0xF0, 0x0F, 0x0F, 0xFF, 0xF0}, false},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprint(tc.tolerate), func(t *testing.T) {
			p := &Params{
				Columns:                8,
				EndOfLine:              true,
				DamagedRowsBeforeError: tc.tolerate,
			}
			got, err := decode(t, p, b.data)
			if (err != nil) != tc.wantErr {
				t.Errorf("error = %v, want error: %t", err, tc.wantErr)
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("wrong rows (-want +got):\n%s", diff)
			}
		})
	}
}

// TestDamagedRowsNotTolerated checks that DamagedRowsBeforeError has no
// effect without EOL codes or for Group 4.
func TestDamagedRowsNotTolerated(t *testing.T) {
	var b bitBuf
	b.write(rowF0 + broken + rowF0)
	for _, k := range []int{-1, 0} {
		p := &Params{
			Columns:                8,
			K:                      k,
			EndOfLine:              k < 0,
			DamagedRowsBeforeError: 5,
		}
		if _, err := decode(t, p, b.data); err == nil {
			t.Errorf("K=%d: expected error", k)
		}
	}
}

// TestPrematureEOL checks that an EOL inside a row counts as damage
// rather than yielding a short row.
func TestPrematureEOL(t *testing.T) {
	var b bitBuf
	b.write(eol + "1011" + eol + rowF0)
	p := &Params{Columns: 8, EndOfLine: true}
	got, err := decode(t, p, b.data)
	if err == nil {
		t.Error("expected error")
	}
	if len(got) != 0 {
		t.Errorf("got %x, want no rows", got)
	}

	p.DamagedRowsBeforeError = 1
	got, err = decode(t, p, b.data)
	if err != nil {
		t.Error(err)
	}
	if diff := cmp.Diff([]byte{0xFF, 0xF0}, got); diff != "" {
		t.Errorf("wrong rows (-want +got):\n%s", diff)
	}
}

// TestTruncatedInput checks that a row cut short by the end of the input
// is completed with white pixels and that no error is reported.
func TestTruncatedInput(t *testing.T) {
	for _, k := range []int{-1, 0, 2} {
		t.Run(fmt.Sprint(k), func(t *testing.T) {
			p := &Params{Columns: 16, K: k, IgnoreEndOfBlock: true}
			data := encode(t, p, testImage)
			// cut inside the last row
			data = data[:len(data)-1]
			got, err := decode(t, p, data)
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != len(testImage) {
				t.Fatalf("got %d bytes, want %d", len(got), len(testImage))
			}
			if diff := cmp.Diff(testImage[:len(testImage)-2], got[:len(got)-2]); diff != "" {
				t.Errorf("leading rows differ (-want +got):\n%s", diff)
			}
		})
	}
}

// TestGroup4EOL checks that Group 4 data with EOL codes before or after
// each row decodes correctly.
func TestGroup4EOL(t *testing.T) {
	p := &Params{Columns: 16, K: -1, IgnoreEndOfBlock: true}
	plain := encode(t, p, testImage[:4])

	// re-encode with EOL codes and check that both readings agree
	p.EndOfLine = true
	withEOL := encode(t, p, testImage[:4])
	if bytes.Equal(plain, withEOL) {
		t.Fatal("writer omits EOL codes for K < 0")
	}
	for _, flag := range []bool{false, true} {
		p.EndOfLine = flag
		got, err := decode(t, p, withEOL)
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(testImage[:4], got); diff != "" {
			t.Errorf("EndOfLine=%t (-want +got):\n%s", flag, diff)
		}
	}
}

// TestRowsBeforeEOFB checks that the end-of-block pattern ends decoding
// before MaxRows is reached.
func TestRowsBeforeEOFB(t *testing.T) {
	for _, k := range []int{-1, 0, 2} {
		p := &Params{Columns: 16, K: k, EndOfLine: true}
		data := encode(t, p, testImage)

		// an EOFB/RTC follows the data; the reader must stop there even
		// when MaxRows is larger
		p.MaxRows = 100
		got, err := decode(t, p, append(data, 0xff, 0xff))
		if err != nil {
			t.Fatalf("K=%d: %v", k, err)
		}
		if diff := cmp.Diff(testImage, got); diff != "" {
			t.Errorf("K=%d (-want +got):\n%s", k, diff)
		}
	}
}

// FuzzReadWriteRead checks that the reader neither panics nor hangs on
// arbitrary input, and that whatever it decodes survives a write-read
// cycle unchanged.
func FuzzReadWriteRead(f *testing.F) {
	for _, k := range []int{-1, 0, 2} {
		for _, flags := range []byte{0, 1, 2, 3, 4, 7} {
			p := &Params{Columns: 16, K: k, EndOfLine: flags&1 != 0,
				EncodedByteAlign: flags&2 != 0, IgnoreEndOfBlock: flags&4 != 0}
			f.Add(byte(k+1), flags, encode(f, p, testImage))
		}
	}
	f.Fuzz(func(t *testing.T, kByte, flags byte, data []byte) {
		p := &Params{
			Columns:                int(flags>>4) + 1,
			K:                      int(kByte%3) - 1,
			MaxRows:                64,
			EndOfLine:              flags&1 != 0,
			EncodedByteAlign:       flags&2 != 0,
			IgnoreEndOfBlock:       flags&4 != 0,
			BlackIs1:               flags&8 != 0,
			DamagedRowsBeforeError: int(kByte / 3),
		}
		r, err := NewReader(bytes.NewReader(data), p)
		if err != nil {
			t.Fatal(err)
		}
		first, _ := io.ReadAll(r)
		if len(first) == 0 {
			return
		}

		p.MaxRows = 0
		p.DamagedRowsBeforeError = 0
		encoded := encode(t, p, first)
		second, err := decode(t, p, encoded)
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(first, second); diff != "" {
			t.Errorf("round trip failed (-want +got):\n%s", diff)
		}
	})
}
