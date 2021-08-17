// seehuhn.de/go/pdf - support for reading and writing PDF files
// Copyright (C) 2021  Jochen Voss <voss@seehuhn.de>
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

package font

import "unicode"

// Encoding describes the correspondence between character indices and unicode
// characters for a PDF font.  The Decode() function returns
// unicode.ReplacementChar for undefined code points.
type Encoding interface {
	Decode(c byte) rune
	Encode(r rune) (byte, bool)
}

var (
	// StandardEncoding is one of the PDF Latin-text encodings.
	// It is normally only used for metadata, but not as a character
	// encoding for glyphs in a font.
	// See PDF 32000-1:2008, Table D.2
	StandardEncoding = &tables{toStandard, fromStandard}

	// MacRomanEncoding is one of the PDF Latin-text encodings.
	// See PDF 32000-1:2008, Table D.2
	MacRomanEncoding = &tables{toMacRoman, fromMacRoman}

	// WinAnsiEncoding is one of the PDF Latin-text encodings.
	// See PDF 32000-1:2008, Table D.2
	WinAnsiEncoding = &tables{toWinAnsi, fromWinAnsi}

	// TODO(voss): add the MacExpertEncoding
)

type tables struct {
	To   map[rune]byte
	From []rune
}

func (enc *tables) Encode(r rune) (byte, bool) {
	c, ok := enc.To[r]
	if ok {
		return c, true
	}
	if r <= 255 && enc.From[r] == r {
		return byte(r), true
	}
	return 0, false
}

func (enc *tables) Decode(c byte) rune {
	return enc.From[c]
}

var fromStandard = []rune{
	noRune, noRune, noRune, noRune, noRune, noRune, noRune, noRune,
	noRune, noRune, noRune, noRune, noRune, noRune, noRune, noRune,
	noRune, noRune, noRune, noRune, noRune, noRune, noRune, noRune,
	noRune, noRune, noRune, noRune, noRune, noRune, noRune, noRune,
	0x0020, 0x0021, 0x0022, 0x0023, 0x0024, 0x0025, 0x0026, 0x2019,
	0x0028, 0x0029, 0x002a, 0x002b, 0x002c, 0x002d, 0x002e, 0x002f,
	0x0030, 0x0031, 0x0032, 0x0033, 0x0034, 0x0035, 0x0036, 0x0037,
	0x0038, 0x0039, 0x003a, 0x003b, 0x003c, 0x003d, 0x003e, 0x003f,
	0x0040, 0x0041, 0x0042, 0x0043, 0x0044, 0x0045, 0x0046, 0x0047,
	0x0048, 0x0049, 0x004a, 0x004b, 0x004c, 0x004d, 0x004e, 0x004f,
	0x0050, 0x0051, 0x0052, 0x0053, 0x0054, 0x0055, 0x0056, 0x0057,
	0x0058, 0x0059, 0x005a, 0x005b, 0x005c, 0x005d, 0x005e, 0x005f,
	0x2018, 0x0061, 0x0062, 0x0063, 0x0064, 0x0065, 0x0066, 0x0067,
	0x0068, 0x0069, 0x006a, 0x006b, 0x006c, 0x006d, 0x006e, 0x006f,
	0x0070, 0x0071, 0x0072, 0x0073, 0x0074, 0x0075, 0x0076, 0x0077,
	0x0078, 0x0079, 0x007a, 0x007b, 0x007c, 0x007d, 0x007e, noRune,
	noRune, noRune, noRune, noRune, noRune, noRune, noRune, noRune,
	noRune, noRune, noRune, noRune, noRune, noRune, noRune, noRune,
	noRune, noRune, noRune, noRune, noRune, noRune, noRune, noRune,
	noRune, noRune, noRune, noRune, noRune, noRune, noRune, noRune,
	noRune, 0x00a1, 0x00a2, 0x00a3, 0x2044, 0x00a5, 0x0192, 0x00a7,
	0x00a4, 0x0027, 0x201c, 0x00ab, 0x2039, 0x203a, 0xfb01, 0xfb02,
	noRune, 0x2013, 0x2020, 0x2021, 0x00b7, noRune, 0x00b6, 0x2022,
	0x201a, 0x201e, 0x201d, 0x00bb, 0x2026, 0x2030, noRune, 0x00bf,
	noRune, 0x0060, 0x00b4, 0x02c6, 0x02dc, 0x00af, 0x02d8, 0x02d9,
	0x00a8, noRune, 0x02da, 0x00b8, noRune, 0x02dd, 0x02db, 0x02c7,
	0x2014, noRune, noRune, noRune, noRune, noRune, noRune, noRune,
	noRune, noRune, noRune, noRune, noRune, noRune, noRune, noRune,
	noRune, 0x00c6, noRune, 0x00aa, noRune, noRune, noRune, noRune,
	0x0141, 0x00d8, 0x0152, 0x00ba, noRune, noRune, noRune, noRune,
	noRune, 0x00e6, noRune, noRune, noRune, 0x0131, noRune, noRune,
	0x0142, 0x00f8, 0x0153, 0x00df, noRune, noRune, noRune, noRune,
}

var toStandard = map[rune]byte{
	0x2019: 39,
	0x2018: 96,
	0x2044: 164,
	0x0192: 166,
	0x00a4: 168,
	0x0027: 169,
	0x201c: 170,
	0x2039: 172,
	0x203a: 173,
	0xfb01: 174,
	0xfb02: 175,
	0x2013: 177,
	0x2020: 178,
	0x2021: 179,
	0x00b7: 180,
	0x2022: 183,
	0x201a: 184,
	0x201e: 185,
	0x201d: 186,
	0x2026: 188,
	0x2030: 189,
	0x0060: 193,
	0x00b4: 194,
	0x02c6: 195,
	0x02dc: 196,
	0x00af: 197,
	0x02d8: 198,
	0x02d9: 199,
	0x00a8: 200,
	0x02da: 202,
	0x00b8: 203,
	0x02dd: 205,
	0x02db: 206,
	0x02c7: 207,
	0x2014: 208,
	0x00c6: 225,
	0x00aa: 227,
	0x0141: 232,
	0x00d8: 233,
	0x0152: 234,
	0x00ba: 235,
	0x00e6: 241,
	0x0131: 245,
	0x0142: 248,
	0x00f8: 249,
	0x0153: 250,
	0x00df: 251,
}

var fromMacRoman = []rune{
	noRune, noRune, noRune, noRune, noRune, noRune, noRune, noRune,
	noRune, noRune, noRune, noRune, noRune, noRune, noRune, noRune,
	noRune, noRune, noRune, noRune, noRune, noRune, noRune, noRune,
	noRune, noRune, noRune, noRune, noRune, noRune, noRune, noRune,
	0x0020, 0x0021, 0x0022, 0x0023, 0x0024, 0x0025, 0x0026, 0x0027,
	0x0028, 0x0029, 0x002a, 0x002b, 0x002c, 0x002d, 0x002e, 0x002f,
	0x0030, 0x0031, 0x0032, 0x0033, 0x0034, 0x0035, 0x0036, 0x0037,
	0x0038, 0x0039, 0x003a, 0x003b, 0x003c, 0x003d, 0x003e, 0x003f,
	0x0040, 0x0041, 0x0042, 0x0043, 0x0044, 0x0045, 0x0046, 0x0047,
	0x0048, 0x0049, 0x004a, 0x004b, 0x004c, 0x004d, 0x004e, 0x004f,
	0x0050, 0x0051, 0x0052, 0x0053, 0x0054, 0x0055, 0x0056, 0x0057,
	0x0058, 0x0059, 0x005a, 0x005b, 0x005c, 0x005d, 0x005e, 0x005f,
	0x0060, 0x0061, 0x0062, 0x0063, 0x0064, 0x0065, 0x0066, 0x0067,
	0x0068, 0x0069, 0x006a, 0x006b, 0x006c, 0x006d, 0x006e, 0x006f,
	0x0070, 0x0071, 0x0072, 0x0073, 0x0074, 0x0075, 0x0076, 0x0077,
	0x0078, 0x0079, 0x007a, 0x007b, 0x007c, 0x007d, 0x007e, noRune,
	0x00c4, 0x00c5, 0x00c7, 0x00c9, 0x00d1, 0x00d6, 0x00dc, 0x00e1,
	0x00e0, 0x00e2, 0x00e4, 0x00e3, 0x00e5, 0x00e7, 0x00e9, 0x00e8,
	0x00ea, 0x00eb, 0x00ed, 0x00ec, 0x00ee, 0x00ef, 0x00f1, 0x00f3,
	0x00f2, 0x00f4, 0x00f6, 0x00f5, 0x00fa, 0x00f9, 0x00fb, 0x00fc,
	0x2020, 0x00b0, 0x00a2, 0x00a3, 0x00a7, 0x2022, 0x00b6, 0x00df,
	0x00ae, 0x00a9, 0x2122, 0x00b4, 0x00a8, noRune, 0x00c6, 0x00d8,
	noRune, 0x00b1, noRune, noRune, 0x00a5, 0x00b5, noRune, noRune,
	noRune, noRune, noRune, 0x00aa, 0x00ba, noRune, 0x00e6, 0x00f8,
	0x00bf, 0x00a1, 0x00ac, noRune, 0x0192, noRune, noRune, 0x00ab,
	0x00bb, 0x2026, 0x00a0, 0x00c0, 0x00c3, 0x00d5, 0x0152, 0x0153,
	0x2013, 0x2014, 0x201c, 0x201d, 0x2018, 0x2019, 0x00f7, noRune,
	0x00ff, 0x0178, 0x2044, 0x00a4, 0x2039, 0x203a, 0xfb01, 0xfb02,
	0x2021, 0x00b7, 0x201a, 0x201e, 0x2030, 0x00c2, 0x00ca, 0x00c1,
	0x00cb, 0x00c8, 0x00cd, 0x00ce, 0x00cf, 0x00cc, 0x00d3, 0x00d4,
	noRune, 0x00d2, 0x00da, 0x00db, 0x00d9, 0x0131, 0x02c6, 0x02dc,
	0x00af, 0x02d8, 0x02d9, 0x02da, 0x00b8, 0x02dd, 0x02db, 0x02c7,
}

var toMacRoman = map[rune]byte{
	0x00c4: 128,
	0x00c5: 129,
	0x00c7: 130,
	0x00c9: 131,
	0x00d1: 132,
	0x00d6: 133,
	0x00dc: 134,
	0x00e1: 135,
	0x00e0: 136,
	0x00e2: 137,
	0x00e4: 138,
	0x00e3: 139,
	0x00e5: 140,
	0x00e7: 141,
	0x00e9: 142,
	0x00e8: 143,
	0x00ea: 144,
	0x00eb: 145,
	0x00ed: 146,
	0x00ec: 147,
	0x00ee: 148,
	0x00ef: 149,
	0x00f1: 150,
	0x00f3: 151,
	0x00f2: 152,
	0x00f4: 153,
	0x00f6: 154,
	0x00f5: 155,
	0x00fa: 156,
	0x00f9: 157,
	0x00fb: 158,
	0x00fc: 159,
	0x2020: 160,
	0x00b0: 161,
	0x00a7: 164,
	0x2022: 165,
	0x00b6: 166,
	0x00df: 167,
	0x00ae: 168,
	0x2122: 170,
	0x00b4: 171,
	0x00a8: 172,
	0x00c6: 174,
	0x00d8: 175,
	0x00a5: 180,
	0x00aa: 187,
	0x00ba: 188,
	0x00e6: 190,
	0x00f8: 191,
	0x00bf: 192,
	0x00a1: 193,
	0x00ac: 194,
	0x0192: 196,
	0x00ab: 199,
	0x00bb: 200,
	0x2026: 201,
	0x00a0: 202,
	0x00c0: 203,
	0x00c3: 204,
	0x00d5: 205,
	0x0152: 206,
	0x0153: 207,
	0x2013: 208,
	0x2014: 209,
	0x201c: 210,
	0x201d: 211,
	0x2018: 212,
	0x2019: 213,
	0x00f7: 214,
	0x00ff: 216,
	0x0178: 217,
	0x2044: 218,
	0x00a4: 219,
	0x2039: 220,
	0x203a: 221,
	0xfb01: 222,
	0xfb02: 223,
	0x2021: 224,
	0x00b7: 225,
	0x201a: 226,
	0x201e: 227,
	0x2030: 228,
	0x00c2: 229,
	0x00ca: 230,
	0x00c1: 231,
	0x00cb: 232,
	0x00c8: 233,
	0x00cd: 234,
	0x00ce: 235,
	0x00cf: 236,
	0x00cc: 237,
	0x00d3: 238,
	0x00d4: 239,
	0x00d2: 241,
	0x00da: 242,
	0x00db: 243,
	0x00d9: 244,
	0x0131: 245,
	0x02c6: 246,
	0x02dc: 247,
	0x00af: 248,
	0x02d8: 249,
	0x02d9: 250,
	0x02da: 251,
	0x00b8: 252,
	0x02dd: 253,
	0x02db: 254,
	0x02c7: 255,
}

var fromWinAnsi = []rune{
	noRune, noRune, noRune, noRune, noRune, noRune, noRune, noRune,
	noRune, noRune, noRune, noRune, noRune, noRune, noRune, noRune,
	noRune, noRune, noRune, noRune, noRune, noRune, noRune, noRune,
	noRune, noRune, noRune, noRune, noRune, noRune, noRune, noRune,
	0x0020, 0x0021, 0x0022, 0x0023, 0x0024, 0x0025, 0x0026, 0x0027,
	0x0028, 0x0029, 0x002a, 0x002b, 0x002c, 0x002d, 0x002e, 0x002f,
	0x0030, 0x0031, 0x0032, 0x0033, 0x0034, 0x0035, 0x0036, 0x0037,
	0x0038, 0x0039, 0x003a, 0x003b, 0x003c, 0x003d, 0x003e, 0x003f,
	0x0040, 0x0041, 0x0042, 0x0043, 0x0044, 0x0045, 0x0046, 0x0047,
	0x0048, 0x0049, 0x004a, 0x004b, 0x004c, 0x004d, 0x004e, 0x004f,
	0x0050, 0x0051, 0x0052, 0x0053, 0x0054, 0x0055, 0x0056, 0x0057,
	0x0058, 0x0059, 0x005a, 0x005b, 0x005c, 0x005d, 0x005e, 0x005f,
	0x0060, 0x0061, 0x0062, 0x0063, 0x0064, 0x0065, 0x0066, 0x0067,
	0x0068, 0x0069, 0x006a, 0x006b, 0x006c, 0x006d, 0x006e, 0x006f,
	0x0070, 0x0071, 0x0072, 0x0073, 0x0074, 0x0075, 0x0076, 0x0077,
	0x0078, 0x0079, 0x007a, 0x007b, 0x007c, 0x007d, 0x007e, noRune,
	0x20ac, noRune, 0x201a, 0x0192, 0x201e, 0x2026, 0x2020, 0x2021,
	0x02c6, 0x2030, 0x0160, 0x2039, 0x0152, noRune, 0x017d, noRune,
	noRune, 0x2018, 0x2019, 0x201c, 0x201d, 0x2022, 0x2013, 0x2014,
	0x02dc, 0x2122, 0x0161, 0x203a, 0x0153, noRune, 0x017e, 0x0178,
	0x00a0, 0x00a1, 0x00a2, 0x00a3, 0x00a4, 0x00a5, 0x00a6, 0x00a7,
	0x00a8, 0x00a9, 0x00aa, 0x00ab, 0x00ac, 0x00ad, 0x00ae, 0x00af,
	0x00b0, 0x00b1, 0x00b2, 0x00b3, 0x00b4, 0x00b5, 0x00b6, 0x00b7,
	0x00b8, 0x00b9, 0x00ba, 0x00bb, 0x00bc, 0x00bd, 0x00be, 0x00bf,
	0x00c0, 0x00c1, 0x00c2, 0x00c3, 0x00c4, 0x00c5, 0x00c6, 0x00c7,
	0x00c8, 0x00c9, 0x00ca, 0x00cb, 0x00cc, 0x00cd, 0x00ce, 0x00cf,
	0x00d0, 0x00d1, 0x00d2, 0x00d3, 0x00d4, 0x00d5, 0x00d6, 0x00d7,
	0x00d8, 0x00d9, 0x00da, 0x00db, 0x00dc, 0x00dd, 0x00de, 0x00df,
	0x00e0, 0x00e1, 0x00e2, 0x00e3, 0x00e4, 0x00e5, 0x00e6, 0x00e7,
	0x00e8, 0x00e9, 0x00ea, 0x00eb, 0x00ec, 0x00ed, 0x00ee, 0x00ef,
	0x00f0, 0x00f1, 0x00f2, 0x00f3, 0x00f4, 0x00f5, 0x00f6, 0x00f7,
	0x00f8, 0x00f9, 0x00fa, 0x00fb, 0x00fc, 0x00fd, 0x00fe, 0x00ff,
}

var toWinAnsi = map[rune]byte{
	0x20ac: 128,
	0x201a: 130,
	0x0192: 131,
	0x201e: 132,
	0x2026: 133,
	0x2020: 134,
	0x2021: 135,
	0x02c6: 136,
	0x2030: 137,
	0x0160: 138,
	0x2039: 139,
	0x0152: 140,
	0x017d: 142,
	0x2018: 145,
	0x2019: 146,
	0x201c: 147,
	0x201d: 148,
	0x2022: 149,
	0x2013: 150,
	0x2014: 151,
	0x02dc: 152,
	0x2122: 153,
	0x0161: 154,
	0x203a: 155,
	0x0153: 156,
	0x017e: 158,
	0x0178: 159,
	0x00a0: 160,
	0x00ad: 173,
}

const noRune = unicode.ReplacementChar
