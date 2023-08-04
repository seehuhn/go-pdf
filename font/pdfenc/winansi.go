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

// Code generated - DO NOT EDIT.

package pdfenc

// WinAnsiEncoding is the PDF version of the standard Microsoft Windows specific
// encoding for Latin text in Western writing systems.
//
// See Appendix D.2 of PDF 32000-1:2008.
var WinAnsiEncoding = [256]string{
	".notdef",        // 0   0x00 \000
	".notdef",        // 1   0x01 \001
	".notdef",        // 2   0x02 \002
	".notdef",        // 3   0x03 \003
	".notdef",        // 4   0x04 \004
	".notdef",        // 5   0x05 \005
	".notdef",        // 6   0x06 \006
	".notdef",        // 7   0x07 \007
	".notdef",        // 8   0x08 \010
	".notdef",        // 9   0x09 \011
	".notdef",        // 10  0x0a \012
	".notdef",        // 11  0x0b \013
	".notdef",        // 12  0x0c \014
	".notdef",        // 13  0x0d \015
	".notdef",        // 14  0x0e \016
	".notdef",        // 15  0x0f \017
	".notdef",        // 16  0x10 \020
	".notdef",        // 17  0x11 \021
	".notdef",        // 18  0x12 \022
	".notdef",        // 19  0x13 \023
	".notdef",        // 20  0x14 \024
	".notdef",        // 21  0x15 \025
	".notdef",        // 22  0x16 \026
	".notdef",        // 23  0x17 \027
	".notdef",        // 24  0x18 \030
	".notdef",        // 25  0x19 \031
	".notdef",        // 26  0x1a \032
	".notdef",        // 27  0x1b \033
	".notdef",        // 28  0x1c \034
	".notdef",        // 29  0x1d \035
	".notdef",        // 30  0x1e \036
	".notdef",        // 31  0x1f \037
	"space",          // 32  0x20 \040 " "
	"exclam",         // 33  0x21 \041 "!"
	"quotedbl",       // 34  0x22 \042 "\""
	"numbersign",     // 35  0x23 \043 "#"
	"dollar",         // 36  0x24 \044 "$"
	"percent",        // 37  0x25 \045 "%"
	"ampersand",      // 38  0x26 \046 "&"
	"quotesingle",    // 39  0x27 \047 "'"
	"parenleft",      // 40  0x28 \050 "("
	"parenright",     // 41  0x29 \051 ")"
	"asterisk",       // 42  0x2a \052 "*"
	"plus",           // 43  0x2b \053 "+"
	"comma",          // 44  0x2c \054 ","
	"hyphen",         // 45  0x2d \055 "-"
	"period",         // 46  0x2e \056 "."
	"slash",          // 47  0x2f \057 "/"
	"zero",           // 48  0x30 \060 "0"
	"one",            // 49  0x31 \061 "1"
	"two",            // 50  0x32 \062 "2"
	"three",          // 51  0x33 \063 "3"
	"four",           // 52  0x34 \064 "4"
	"five",           // 53  0x35 \065 "5"
	"six",            // 54  0x36 \066 "6"
	"seven",          // 55  0x37 \067 "7"
	"eight",          // 56  0x38 \070 "8"
	"nine",           // 57  0x39 \071 "9"
	"colon",          // 58  0x3a \072 ":"
	"semicolon",      // 59  0x3b \073 ";"
	"less",           // 60  0x3c \074 "<"
	"equal",          // 61  0x3d \075 "="
	"greater",        // 62  0x3e \076 ">"
	"question",       // 63  0x3f \077 "?"
	"at",             // 64  0x40 \100 "@"
	"A",              // 65  0x41 \101 "A"
	"B",              // 66  0x42 \102 "B"
	"C",              // 67  0x43 \103 "C"
	"D",              // 68  0x44 \104 "D"
	"E",              // 69  0x45 \105 "E"
	"F",              // 70  0x46 \106 "F"
	"G",              // 71  0x47 \107 "G"
	"H",              // 72  0x48 \110 "H"
	"I",              // 73  0x49 \111 "I"
	"J",              // 74  0x4a \112 "J"
	"K",              // 75  0x4b \113 "K"
	"L",              // 76  0x4c \114 "L"
	"M",              // 77  0x4d \115 "M"
	"N",              // 78  0x4e \116 "N"
	"O",              // 79  0x4f \117 "O"
	"P",              // 80  0x50 \120 "P"
	"Q",              // 81  0x51 \121 "Q"
	"R",              // 82  0x52 \122 "R"
	"S",              // 83  0x53 \123 "S"
	"T",              // 84  0x54 \124 "T"
	"U",              // 85  0x55 \125 "U"
	"V",              // 86  0x56 \126 "V"
	"W",              // 87  0x57 \127 "W"
	"X",              // 88  0x58 \130 "X"
	"Y",              // 89  0x59 \131 "Y"
	"Z",              // 90  0x5a \132 "Z"
	"bracketleft",    // 91  0x5b \133 "["
	"backslash",      // 92  0x5c \134 "\\"
	"bracketright",   // 93  0x5d \135 "]"
	"asciicircum",    // 94  0x5e \136 "^"
	"underscore",     // 95  0x5f \137 "_"
	"grave",          // 96  0x60 \140 "`"
	"a",              // 97  0x61 \141 "a"
	"b",              // 98  0x62 \142 "b"
	"c",              // 99  0x63 \143 "c"
	"d",              // 100 0x64 \144 "d"
	"e",              // 101 0x65 \145 "e"
	"f",              // 102 0x66 \146 "f"
	"g",              // 103 0x67 \147 "g"
	"h",              // 104 0x68 \150 "h"
	"i",              // 105 0x69 \151 "i"
	"j",              // 106 0x6a \152 "j"
	"k",              // 107 0x6b \153 "k"
	"l",              // 108 0x6c \154 "l"
	"m",              // 109 0x6d \155 "m"
	"n",              // 110 0x6e \156 "n"
	"o",              // 111 0x6f \157 "o"
	"p",              // 112 0x70 \160 "p"
	"q",              // 113 0x71 \161 "q"
	"r",              // 114 0x72 \162 "r"
	"s",              // 115 0x73 \163 "s"
	"t",              // 116 0x74 \164 "t"
	"u",              // 117 0x75 \165 "u"
	"v",              // 118 0x76 \166 "v"
	"w",              // 119 0x77 \167 "w"
	"x",              // 120 0x78 \170 "x"
	"y",              // 121 0x79 \171 "y"
	"z",              // 122 0x7a \172 "z"
	"braceleft",      // 123 0x7b \173 "{"
	"bar",            // 124 0x7c \174 "|"
	"braceright",     // 125 0x7d \175 "}"
	"asciitilde",     // 126 0x7e \176 "~"
	".notdef",        // 127 0x7f \177
	"Euro",           // 128 0x80 \200 "€"
	".notdef",        // 129 0x81 \201
	"quotesinglbase", // 130 0x82 \202 "‚"
	"florin",         // 131 0x83 \203 "ƒ"
	"quotedblbase",   // 132 0x84 \204 "„"
	"ellipsis",       // 133 0x85 \205 "…"
	"dagger",         // 134 0x86 \206 "†"
	"daggerdbl",      // 135 0x87 \207 "‡"
	"circumflex",     // 136 0x88 \210 "ˆ"
	"perthousand",    // 137 0x89 \211 "‰"
	"Scaron",         // 138 0x8a \212 "Š"
	"guilsinglleft",  // 139 0x8b \213 "‹"
	"OE",             // 140 0x8c \214 "Œ"
	".notdef",        // 141 0x8d \215
	"Zcaron",         // 142 0x8e \216 "Ž"
	".notdef",        // 143 0x8f \217
	".notdef",        // 144 0x90 \220
	"quoteleft",      // 145 0x91 \221 "‘"
	"quoteright",     // 146 0x92 \222 "’"
	"quotedblleft",   // 147 0x93 \223 "“"
	"quotedblright",  // 148 0x94 \224 "”"
	"bullet",         // 149 0x95 \225 "•"
	"endash",         // 150 0x96 \226 "–"
	"emdash",         // 151 0x97 \227 "—"
	"tilde",          // 152 0x98 \230 "˜"
	"trademark",      // 153 0x99 \231 "™"
	"scaron",         // 154 0x9a \232 "š"
	"guilsinglright", // 155 0x9b \233 "›"
	"oe",             // 156 0x9c \234 "œ"
	".notdef",        // 157 0x9d \235
	"zcaron",         // 158 0x9e \236 "ž"
	"Ydieresis",      // 159 0x9f \237 "Ÿ"
	"space",          // 160 0xa0 \240 " "
	"exclamdown",     // 161 0xa1 \241 "¡"
	"cent",           // 162 0xa2 \242 "¢"
	"sterling",       // 163 0xa3 \243 "£"
	"currency",       // 164 0xa4 \244 "¤"
	"yen",            // 165 0xa5 \245 "¥"
	"brokenbar",      // 166 0xa6 \246 "¦"
	"section",        // 167 0xa7 \247 "§"
	"dieresis",       // 168 0xa8 \250 "¨"
	"copyright",      // 169 0xa9 \251 "©"
	"ordfeminine",    // 170 0xaa \252 "ª"
	"guillemotleft",  // 171 0xab \253 "«"
	"logicalnot",     // 172 0xac \254 "¬"
	"hyphen",         // 173 0xad \255 "-"
	"registered",     // 174 0xae \256 "®"
	"macron",         // 175 0xaf \257 "¯"
	"degree",         // 176 0xb0 \260 "°"
	"plusminus",      // 177 0xb1 \261 "±"
	"twosuperior",    // 178 0xb2 \262 "²"
	"threesuperior",  // 179 0xb3 \263 "³"
	"acute",          // 180 0xb4 \264 "´"
	"mu",             // 181 0xb5 \265 "µ"
	"paragraph",      // 182 0xb6 \266 "¶"
	"periodcentered", // 183 0xb7 \267 "·"
	"cedilla",        // 184 0xb8 \270 "¸"
	"onesuperior",    // 185 0xb9 \271 "¹"
	"ordmasculine",   // 186 0xba \272 "º"
	"guillemotright", // 187 0xbb \273 "»"
	"onequarter",     // 188 0xbc \274 "¼"
	"onehalf",        // 189 0xbd \275 "½"
	"threequarters",  // 190 0xbe \276 "¾"
	"questiondown",   // 191 0xbf \277 "¿"
	"Agrave",         // 192 0xc0 \300 "À"
	"Aacute",         // 193 0xc1 \301 "Á"
	"Acircumflex",    // 194 0xc2 \302 "Â"
	"Atilde",         // 195 0xc3 \303 "Ã"
	"Adieresis",      // 196 0xc4 \304 "Ä"
	"Aring",          // 197 0xc5 \305 "Å"
	"AE",             // 198 0xc6 \306 "Æ"
	"Ccedilla",       // 199 0xc7 \307 "Ç"
	"Egrave",         // 200 0xc8 \310 "È"
	"Eacute",         // 201 0xc9 \311 "É"
	"Ecircumflex",    // 202 0xca \312 "Ê"
	"Edieresis",      // 203 0xcb \313 "Ë"
	"Igrave",         // 204 0xcc \314 "Ì"
	"Iacute",         // 205 0xcd \315 "Í"
	"Icircumflex",    // 206 0xce \316 "Î"
	"Idieresis",      // 207 0xcf \317 "Ï"
	"Eth",            // 208 0xd0 \320 "Ð"
	"Ntilde",         // 209 0xd1 \321 "Ñ"
	"Ograve",         // 210 0xd2 \322 "Ò"
	"Oacute",         // 211 0xd3 \323 "Ó"
	"Ocircumflex",    // 212 0xd4 \324 "Ô"
	"Otilde",         // 213 0xd5 \325 "Õ"
	"Odieresis",      // 214 0xd6 \326 "Ö"
	"multiply",       // 215 0xd7 \327 "×"
	"Oslash",         // 216 0xd8 \330 "Ø"
	"Ugrave",         // 217 0xd9 \331 "Ù"
	"Uacute",         // 218 0xda \332 "Ú"
	"Ucircumflex",    // 219 0xdb \333 "Û"
	"Udieresis",      // 220 0xdc \334 "Ü"
	"Yacute",         // 221 0xdd \335 "Ý"
	"Thorn",          // 222 0xde \336 "Þ"
	"germandbls",     // 223 0xdf \337 "ß"
	"agrave",         // 224 0xe0 \340 "à"
	"aacute",         // 225 0xe1 \341 "á"
	"acircumflex",    // 226 0xe2 \342 "â"
	"atilde",         // 227 0xe3 \343 "ã"
	"adieresis",      // 228 0xe4 \344 "ä"
	"aring",          // 229 0xe5 \345 "å"
	"ae",             // 230 0xe6 \346 "æ"
	"ccedilla",       // 231 0xe7 \347 "ç"
	"egrave",         // 232 0xe8 \350 "è"
	"eacute",         // 233 0xe9 \351 "é"
	"ecircumflex",    // 234 0xea \352 "ê"
	"edieresis",      // 235 0xeb \353 "ë"
	"igrave",         // 236 0xec \354 "ì"
	"iacute",         // 237 0xed \355 "í"
	"icircumflex",    // 238 0xee \356 "î"
	"idieresis",      // 239 0xef \357 "ï"
	"eth",            // 240 0xf0 \360 "ð"
	"ntilde",         // 241 0xf1 \361 "ñ"
	"ograve",         // 242 0xf2 \362 "ò"
	"oacute",         // 243 0xf3 \363 "ó"
	"ocircumflex",    // 244 0xf4 \364 "ô"
	"otilde",         // 245 0xf5 \365 "õ"
	"odieresis",      // 246 0xf6 \366 "ö"
	"divide",         // 247 0xf7 \367 "÷"
	"oslash",         // 248 0xf8 \370 "ø"
	"ugrave",         // 249 0xf9 \371 "ù"
	"uacute",         // 250 0xfa \372 "ú"
	"ucircumflex",    // 251 0xfb \373 "û"
	"udieresis",      // 252 0xfc \374 "ü"
	"yacute",         // 253 0xfd \375 "ý"
	"thorn",          // 254 0xfe \376 "þ"
	"ydieresis",      // 255 0xff \377 "ÿ"
}
