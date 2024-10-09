// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2024  Jochen Voss <voss@seehuhn.de>
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

package encoding

import (
	"fmt"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/pdfenc"
)

var testBuiltinEncoding = []string{
	".notdef",        // 0
	".notdef",        // 1
	".notdef",        // 2
	".notdef",        // 3
	".notdef",        // 4
	".notdef",        // 5
	".notdef",        // 6
	".notdef",        // 7
	".notdef",        // 8
	".notdef",        // 9
	".notdef",        // 10
	".notdef",        // 11
	".notdef",        // 12
	".notdef",        // 13
	".notdef",        // 14
	".notdef",        // 15
	".notdef",        // 16
	".notdef",        // 17
	".notdef",        // 18
	".notdef",        // 19
	".notdef",        // 20
	".notdef",        // 21
	".notdef",        // 22
	".notdef",        // 23
	".notdef",        // 24
	".notdef",        // 25
	".notdef",        // 26
	".notdef",        // 27
	".notdef",        // 28
	".notdef",        // 29
	"test1",          // 30
	"test2",          // 31
	"space",          // 32
	"test3",          // 33
	"test4",          // 34
	"numbersign",     // 35
	"dollar",         // 36
	"percent",        // 37
	"ampersand",      // 38
	"quoteright",     // 39
	"parenleft",      // 40
	"parenright",     // 41
	".notdef",        // 42
	".notdef",        // 43
	"comma",          // 44
	"hyphen",         // 45
	"period",         // 46
	"slash",          // 47
	"zero",           // 48
	"one",            // 49
	"two",            // 50
	"three",          // 51
	"four",           // 52
	"five",           // 53
	"six",            // 54
	"seven",          // 55
	"eight",          // 56
	"nine",           // 57
	"colon",          // 58
	"semicolon",      // 59
	"less",           // 60
	"equal",          // 61
	"greater",        // 62
	"question",       // 63
	"at",             // 64
	"moved",          // 65
	"B",              // 66
	"C",              // 67
	"D",              // 68
	"E",              // 69
	"F",              // 70
	"G",              // 71
	"H",              // 72
	"I",              // 73
	"J",              // 74
	"K",              // 75
	"L",              // 76
	"M",              // 77
	"N",              // 78
	"O",              // 79
	"P",              // 80
	"Q",              // 81
	"R",              // 82
	"S",              // 83
	"T",              // 84
	"U",              // 85
	"V",              // 86
	"W",              // 87
	"X",              // 88
	"Y",              // 89
	"Z",              // 90
	"bracketleft",    // 91
	"backslash",      // 92
	"bracketright",   // 93
	"asciicircum",    // 94
	"underscore",     // 95
	"quoteleft",      // 96
	"a",              // 97
	"b",              // 98
	"c",              // 99
	"d",              // 100
	"e",              // 101
	"f",              // 102
	"g",              // 103
	"h",              // 104
	"i",              // 105
	"j",              // 106
	"k",              // 107
	"l",              // 108
	"m",              // 109
	"n",              // 110
	"o",              // 111
	"p",              // 112
	"q",              // 113
	"r",              // 114
	"s",              // 115
	"t",              // 116
	"u",              // 117
	"v",              // 118
	"w",              // 119
	"x",              // 120
	"y",              // 121
	"z",              // 122
	"braceleft",      // 123
	"bar",            // 124
	"braceright",     // 125
	"asciitilde",     // 126
	".notdef",        // 127
	".notdef",        // 128
	".notdef",        // 129
	".notdef",        // 130
	".notdef",        // 131
	".notdef",        // 132
	".notdef",        // 133
	".notdef",        // 134
	".notdef",        // 135
	".notdef",        // 136
	".notdef",        // 137
	".notdef",        // 138
	".notdef",        // 139
	".notdef",        // 140
	".notdef",        // 141
	".notdef",        // 142
	".notdef",        // 143
	".notdef",        // 144
	".notdef",        // 145
	".notdef",        // 146
	".notdef",        // 147
	".notdef",        // 148
	".notdef",        // 149
	".notdef",        // 150
	".notdef",        // 151
	".notdef",        // 152
	".notdef",        // 153
	".notdef",        // 154
	".notdef",        // 155
	".notdef",        // 156
	".notdef",        // 157
	".notdef",        // 158
	".notdef",        // 159
	".notdef",        // 160
	"exclamdown",     // 161
	"cent",           // 162
	"sterling",       // 163
	"fraction",       // 164
	"yen",            // 165
	"florin",         // 166
	"section",        // 167
	"currency",       // 168
	"quotesingle",    // 169
	"quotedblleft",   // 170
	"guillemotleft",  // 171
	"guilsinglleft",  // 172
	"guilsinglright", // 173
	"fi",             // 174
	"fl",             // 175
	".notdef",        // 176
	"endash",         // 177
	"dagger",         // 178
	"daggerdbl",      // 179
	"periodcentered", // 180
	".notdef",        // 181
	"paragraph",      // 182
	"bullet",         // 183
	"quotesinglbase", // 184
	"quotedblbase",   // 185
	"quotedblright",  // 186
	"guillemotright", // 187
	"ellipsis",       // 188
	"perthousand",    // 189
	".notdef",        // 190
	"questiondown",   // 191
	".notdef",        // 192
	"grave",          // 193
	"acute",          // 194
	"circumflex",     // 195
	"tilde",          // 196
	"macron",         // 197
	"breve",          // 198
	"dotaccent",      // 199
	"dieresis",       // 200
	".notdef",        // 201
	"ring",           // 202
	"cedilla",        // 203
	".notdef",        // 204
	"hungarumlaut",   // 205
	"ogonek",         // 206
	"caron",          // 207
	"emdash",         // 208
	".notdef",        // 209
	".notdef",        // 210
	".notdef",        // 211
	".notdef",        // 212
	".notdef",        // 213
	".notdef",        // 214
	".notdef",        // 215
	".notdef",        // 216
	".notdef",        // 217
	".notdef",        // 218
	".notdef",        // 219
	".notdef",        // 220
	".notdef",        // 221
	".notdef",        // 222
	".notdef",        // 223
	".notdef",        // 224
	"A",              // 225
	".notdef",        // 226
	"ordfeminine",    // 227
	".notdef",        // 228
	".notdef",        // 229
	".notdef",        // 230
	".notdef",        // 231
	"Lslash",         // 232
	"Oslash",         // 233
	"OE",             // 234
	"ordmasculine",   // 235
	".notdef",        // 236
	".notdef",        // 237
	".notdef",        // 238
	".notdef",        // 239
	".notdef",        // 240
	"ae",             // 241
	".notdef",        // 242
	".notdef",        // 243
	".notdef",        // 244
	"dotlessi",       // 245
	".notdef",        // 246
	".notdef",        // 247
	"lslash",         // 248
	"oslash",         // 249
	"oe",             // 250
	"germandbls",     // 251
	".notdef",        // 252
	".notdef",        // 253
	".notdef",        // 254
	"ydieresis",      // 255
}

type sampleEncoding struct {
	enc   *Encoding
	names []string
}

func newSampleEncoding() *sampleEncoding {
	return &sampleEncoding{
		enc:   New(),
		names: make([]string, 256),
	}
}

func (e *sampleEncoding) set(code byte, name string) error {
	if name == ".notdef" {
		return nil
	}

	if name != "" {
		e.enc.enc[code] = e.enc.Allocate(name)
		e.names[code] = name
	} else {
		e.enc.UseBuiltinEncoding(code)
	}
	return nil
}

func makeTestEncodings() []*sampleEncoding {
	var res []*sampleEncoding

	special := [][]string{
		testBuiltinEncoding,
		pdfenc.WinAnsi.Encoding[:],
		pdfenc.MacRoman.Encoding[:],
		pdfenc.MacExpert.Encoding[:],
		pdfenc.Standard.Encoding[:],
	}
	unique := make([][]int, len(special))
	for i, names := range special {
		for code := range 256 {
			count := 0
			for j := range special {
				if j == 0 || names[code] != special[j][code] {
					count++
				}
			}
			if count >= len(special)-1 {
				unique[i] = append(unique[i], code)
			}
		}
	}
	for i, names := range special {
		enc := newSampleEncoding()
		for _, code := range unique[i] {
			err := enc.set(byte(code), names[code])
			if err != nil {
				panic(err)
			}
		}
		res = append(res, enc)
	}

	// make an encoding which uses the built-in encoding for some glyphs
	enc := newSampleEncoding()
	for code := range 256 {
		if testBuiltinEncoding[code] != ".notdef" {
			if code%2 == 0 {
				enc.set(byte(code), testBuiltinEncoding[code])
			} else {
				enc.set(byte(code), "")
				enc.names[code] = testBuiltinEncoding[code]
			}
		} else if pdfenc.WinAnsi.Encoding[code] != ".notdef" {
			enc.set(byte(code), pdfenc.WinAnsi.Encoding[code])
		}
	}
	res = append(res, enc)

	return res
}

func TestAsPDF(t *testing.T) {
	const nonSymbolicExt = false
	for _, tp := range "ABC" { // A = Type 1, B = Type 3, C = TrueType
		for i, sample := range makeTestEncodings() {
			t.Run(fmt.Sprintf("case%02d%c", i, tp), func(t *testing.T) {
				enc1 := sample.enc

				canType3 := true
				for code := range 256 {
					cid := enc1.Decode(byte(code))
					if cid == 0 {
						continue
					}
					name := enc1.GlyphName(cid)
					if name == "" {
						canType3 = false
						name = testBuiltinEncoding[code]
					}
					if name != sample.names[code] {
						t.Errorf("CIDName(%d) = %q, want %q", cid, name, sample.names[code])
					}
				}
				if tp == 'B' && !canType3 {
					return
				}

				var enc2 *Encoding
				var obj pdf.Object
				var err error
				switch tp {
				case 'A':
					dicts := &font.Dicts{
						DictType:       font.DictTypeSimpleType1,
						FontDict:       pdf.Dict{},
						FontDescriptor: &font.Descriptor{},
					}
					if !nonSymbolicExt {
						dicts.FontDescriptor.IsSymbolic = true
					}

					obj, err = enc1.AsPDFType1(nonSymbolicExt, pdf.OptPretty|pdf.OptDictTypes)
					if err != nil {
						t.Fatal(err)
					}
					dicts.FontDict["Encoding"] = obj

					enc2, err = ExtractType1(nil, dicts)
					if err != nil {
						t.Fatal(err)
					}
				case 'B':
					obj, err = enc1.AsPDFType3(pdf.OptPretty | pdf.OptDictTypes)
					if err != nil {
						t.Fatal(err)
					}
					enc2, err = ExtractType3(nil, obj)
					if err != nil {
						t.Fatal(err)
					}
				case 'C':
					obj, err := enc1.AsPDFTrueType(testBuiltinEncoding, pdf.OptPretty|pdf.OptDictTypes)
					if err != nil {
						t.Fatal(err)
					}
					enc2, err = ExtractTrueType(nil, obj)
					if err != nil {
						t.Fatal(err)
					}
				}

				hasErrors := false
				for code := range 256 {
					cid1 := enc1.Decode(byte(code))
					if cid1 == 0 {
						continue
					}

					want := sample.names[code]
					cid2 := enc2.Decode(byte(code))
					got := ".notdef"
					if cid2 != 0 {
						got = enc2.GlyphName(cid2)
						if got == "" {
							got = testBuiltinEncoding[code]
						}
					}
					if got != want {
						t.Errorf("code %d -> CID %d -> %q, want %q", code, cid2, enc2.GlyphName(cid2), want)
						hasErrors = true
					}
				}

				if hasErrors {
					t.Log("/Encoding", pdf.AsString(obj))
				}
			})
		}
	}
}
