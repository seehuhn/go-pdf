// seehuhn.de/go/pdf - a library for reading and writing PDF files
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

package cff

// 2-byte string identifier
type sid uint16

type cffStrings struct {
	data []string
	rev  map[string]sid
}

func (ss *cffStrings) Copy() *cffStrings {
	res := &cffStrings{
		data: make([]string, len(ss.data)),
	}
	copy(res.data, ss.data)
	return res
}

func (ss *cffStrings) Len() sid {
	return sid(len(ss.data)) + nStdString
}

func (ss *cffStrings) get(i sid) (string, bool) {
	if i < nStdString {
		return stdStrings[i], true
	}
	i -= nStdString

	if int(i) < len(ss.data) {
		return ss.data[i], true
	}

	return "", false
}

func (ss *cffStrings) lookup(s string) sid {
	if ss.rev == nil {
		ss.rev = make(map[string]sid)
		for i, s := range stdStrings {
			ss.rev[s] = sid(i)
		}
		for i, s := range ss.data {
			ss.rev[s] = sid(i) + nStdString
		}
	}

	res, ok := ss.rev[s]
	if !ok {
		res = sid(len(ss.data))
		ss.data = append(ss.data, s)
		ss.rev[s] = res
	}
	return res
}

func (ss *cffStrings) encode() ([]byte, error) {
	stringIndex := make(cffIndex, 0, len(ss.data))
	for _, s := range ss.data {
		stringIndex = append(stringIndex, []byte(s))
	}
	return stringIndex.encode()
}

var stdStrings = []string{
	".notdef",             // 0
	"space",               // 1
	"exclam",              // 2
	"quotedbl",            // 3
	"numbersign",          // 4
	"dollar",              // 5
	"percent",             // 6
	"ampersand",           // 7
	"quoteright",          // 8
	"parenleft",           // 9
	"parenright",          // 10
	"asterisk",            // 11
	"plus",                // 12
	"comma",               // 13
	"hyphen",              // 14
	"period",              // 15
	"slash",               // 16
	"zero",                // 17
	"one",                 // 18
	"two",                 // 19
	"three",               // 20
	"four",                // 21
	"five",                // 22
	"six",                 // 23
	"seven",               // 24
	"eight",               // 25
	"nine",                // 26
	"colon",               // 27
	"semicolon",           // 28
	"less",                // 29
	"equal",               // 30
	"greater",             // 31
	"question",            // 32
	"at",                  // 33
	"A",                   // 34
	"B",                   // 35
	"C",                   // 36
	"D",                   // 37
	"E",                   // 38
	"F",                   // 39
	"G",                   // 40
	"H",                   // 41
	"I",                   // 42
	"J",                   // 43
	"K",                   // 44
	"L",                   // 45
	"M",                   // 46
	"N",                   // 47
	"O",                   // 48
	"P",                   // 49
	"Q",                   // 50
	"R",                   // 51
	"S",                   // 52
	"T",                   // 53
	"U",                   // 54
	"V",                   // 55
	"W",                   // 56
	"X",                   // 57
	"Y",                   // 58
	"Z",                   // 59
	"bracketleft",         // 60
	"backslash",           // 61
	"bracketright",        // 62
	"asciicircum",         // 63
	"underscore",          // 64
	"quoteleft",           // 65
	"a",                   // 66
	"b",                   // 67
	"c",                   // 68
	"d",                   // 69
	"e",                   // 70
	"f",                   // 71
	"g",                   // 72
	"h",                   // 73
	"i",                   // 74
	"j",                   // 75
	"k",                   // 76
	"l",                   // 77
	"m",                   // 78
	"n",                   // 79
	"o",                   // 80
	"p",                   // 81
	"q",                   // 82
	"r",                   // 83
	"s",                   // 84
	"t",                   // 85
	"u",                   // 86
	"v",                   // 87
	"w",                   // 88
	"x",                   // 89
	"y",                   // 90
	"z",                   // 91
	"braceleft",           // 92
	"bar",                 // 93
	"braceright",          // 94
	"asciitilde",          // 95
	"exclamdown",          // 96
	"cent",                // 97
	"sterling",            // 98
	"fraction",            // 99
	"yen",                 // 100
	"florin",              // 101
	"section",             // 102
	"currency",            // 103
	"quotesingle",         // 104
	"quotedblleft",        // 105
	"guillemotleft",       // 106
	"guilsinglleft",       // 107
	"guilsinglright",      // 108
	"fi",                  // 109
	"fl",                  // 110
	"endash",              // 111
	"dagger",              // 112
	"daggerdbl",           // 113
	"periodcentered",      // 114
	"paragraph",           // 115
	"bullet",              // 116
	"quotesinglbase",      // 117
	"quotedblbase",        // 118
	"quotedblright",       // 119
	"guillemotright",      // 120
	"ellipsis",            // 121
	"perthousand",         // 122
	"questiondown",        // 123
	"grave",               // 124
	"acute",               // 125
	"circumflex",          // 126
	"tilde",               // 127
	"macron",              // 128
	"breve",               // 129
	"dotaccent",           // 130
	"dieresis",            // 131
	"ring",                // 132
	"cedilla",             // 133
	"hungarumlaut",        // 134
	"ogonek",              // 135
	"caron",               // 136
	"emdash",              // 137
	"AE",                  // 138
	"ordfeminine",         // 139
	"Lslash",              // 140
	"Oslash",              // 141
	"OE",                  // 142
	"ordmasculine",        // 143
	"ae",                  // 144
	"dotlessi",            // 145
	"lslash",              // 146
	"oslash",              // 147
	"oe",                  // 148
	"germandbls",          // 149
	"onesuperior",         // 150
	"logicalnot",          // 151
	"mu",                  // 152
	"trademark",           // 153
	"Eth",                 // 154
	"onehalf",             // 155
	"plusminus",           // 156
	"Thorn",               // 157
	"onequarter",          // 158
	"divide",              // 159
	"brokenbar",           // 160
	"degree",              // 161
	"thorn",               // 162
	"threequarters",       // 163
	"twosuperior",         // 164
	"registered",          // 165
	"minus",               // 166
	"eth",                 // 167
	"multiply",            // 168
	"threesuperior",       // 169
	"copyright",           // 170
	"Aacute",              // 171
	"Acircumflex",         // 172
	"Adieresis",           // 173
	"Agrave",              // 174
	"Aring",               // 175
	"Atilde",              // 176
	"Ccedilla",            // 177
	"Eacute",              // 178
	"Ecircumflex",         // 179
	"Edieresis",           // 180
	"Egrave",              // 181
	"Iacute",              // 182
	"Icircumflex",         // 183
	"Idieresis",           // 184
	"Igrave",              // 185
	"Ntilde",              // 186
	"Oacute",              // 187
	"Ocircumflex",         // 188
	"Odieresis",           // 189
	"Ograve",              // 190
	"Otilde",              // 191
	"Scaron",              // 192
	"Uacute",              // 193
	"Ucircumflex",         // 194
	"Udieresis",           // 195
	"Ugrave",              // 196
	"Yacute",              // 197
	"Ydieresis",           // 198
	"Zcaron",              // 199
	"aacute",              // 200
	"acircumflex",         // 201
	"adieresis",           // 202
	"agrave",              // 203
	"aring",               // 204
	"atilde",              // 205
	"ccedilla",            // 206
	"eacute",              // 207
	"ecircumflex",         // 208
	"edieresis",           // 209
	"egrave",              // 210
	"iacute",              // 211
	"icircumflex",         // 212
	"idieresis",           // 213
	"igrave",              // 214
	"ntilde",              // 215
	"oacute",              // 216
	"ocircumflex",         // 217
	"odieresis",           // 218
	"ograve",              // 219
	"otilde",              // 220
	"scaron",              // 221
	"uacute",              // 222
	"ucircumflex",         // 223
	"udieresis",           // 224
	"ugrave",              // 225
	"yacute",              // 226
	"ydieresis",           // 227
	"zcaron",              // 228
	"exclamsmall",         // 229
	"Hungarumlautsmall",   // 230
	"dollaroldstyle",      // 231
	"dollarsuperior",      // 232
	"ampersandsmall",      // 233
	"Acutesmall",          // 234
	"parenleftsuperior",   // 235
	"parenrightsuperior",  // 236
	"twodotenleader",      // 237
	"onedotenleader",      // 238
	"zerooldstyle",        // 239
	"oneoldstyle",         // 240
	"twooldstyle",         // 241
	"threeoldstyle",       // 242
	"fouroldstyle",        // 243
	"fiveoldstyle",        // 244
	"sixoldstyle",         // 245
	"sevenoldstyle",       // 246
	"eightoldstyle",       // 247
	"nineoldstyle",        // 248
	"commasuperior",       // 249
	"threequartersemdash", // 250
	"periodsuperior",      // 251
	"questionsmall",       // 252
	"asuperior",           // 253
	"bsuperior",           // 254
	"centsuperior",        // 255
	"dsuperior",           // 256
	"esuperior",           // 257
	"isuperior",           // 258
	"lsuperior",           // 259
	"msuperior",           // 260
	"nsuperior",           // 261
	"osuperior",           // 262
	"rsuperior",           // 263
	"ssuperior",           // 264
	"tsuperior",           // 265
	"ff",                  // 266
	"ffi",                 // 267
	"ffl",                 // 268
	"parenleftinferior",   // 269
	"parenrightinferior",  // 270
	"Circumflexsmall",     // 271
	"hyphensuperior",      // 272
	"Gravesmall",          // 273
	"Asmall",              // 274
	"Bsmall",              // 275
	"Csmall",              // 276
	"Dsmall",              // 277
	"Esmall",              // 278
	"Fsmall",              // 279
	"Gsmall",              // 280
	"Hsmall",              // 281
	"Ismall",              // 282
	"Jsmall",              // 283
	"Ksmall",              // 284
	"Lsmall",              // 285
	"Msmall",              // 286
	"Nsmall",              // 287
	"Osmall",              // 288
	"Psmall",              // 289
	"Qsmall",              // 290
	"Rsmall",              // 291
	"Ssmall",              // 292
	"Tsmall",              // 293
	"Usmall",              // 294
	"Vsmall",              // 295
	"Wsmall",              // 296
	"Xsmall",              // 297
	"Ysmall",              // 298
	"Zsmall",              // 299
	"colonmonetary",       // 300
	"onefitted",           // 301
	"rupiah",              // 302
	"Tildesmall",          // 303
	"exclamdownsmall",     // 304
	"centoldstyle",        // 305
	"Lslashsmall",         // 306
	"Scaronsmall",         // 307
	"Zcaronsmall",         // 308
	"Dieresissmall",       // 309
	"Brevesmall",          // 310
	"Caronsmall",          // 311
	"Dotaccentsmall",      // 312
	"Macronsmall",         // 313
	"figuredash",          // 314
	"hypheninferior",      // 315
	"Ogoneksmall",         // 316
	"Ringsmall",           // 317
	"Cedillasmall",        // 318
	"questiondownsmall",   // 319
	"oneeighth",           // 320
	"threeeighths",        // 321
	"fiveeighths",         // 322
	"seveneighths",        // 323
	"onethird",            // 324
	"twothirds",           // 325
	"zerosuperior",        // 326
	"foursuperior",        // 327
	"fivesuperior",        // 328
	"sixsuperior",         // 329
	"sevensuperior",       // 330
	"eightsuperior",       // 331
	"ninesuperior",        // 332
	"zeroinferior",        // 333
	"oneinferior",         // 334
	"twoinferior",         // 335
	"threeinferior",       // 336
	"fourinferior",        // 337
	"fiveinferior",        // 338
	"sixinferior",         // 339
	"seveninferior",       // 340
	"eightinferior",       // 341
	"nineinferior",        // 342
	"centinferior",        // 343
	"dollarinferior",      // 344
	"periodinferior",      // 345
	"commainferior",       // 346
	"Agravesmall",         // 347
	"Aacutesmall",         // 348
	"Acircumflexsmall",    // 349
	"Atildesmall",         // 350
	"Adieresissmall",      // 351
	"Aringsmall",          // 352
	"AEsmall",             // 353
	"Ccedillasmall",       // 354
	"Egravesmall",         // 355
	"Eacutesmall",         // 356
	"Ecircumflexsmall",    // 357
	"Edieresissmall",      // 358
	"Igravesmall",         // 359
	"Iacutesmall",         // 360
	"Icircumflexsmall",    // 361
	"Idieresissmall",      // 362
	"Ethsmall",            // 363
	"Ntildesmall",         // 364
	"Ogravesmall",         // 365
	"Oacutesmall",         // 366
	"Ocircumflexsmall",    // 367
	"Otildesmall",         // 368
	"Odieresissmall",      // 369
	"OEsmall",             // 370
	"Oslashsmall",         // 371
	"Ugravesmall",         // 372
	"Uacutesmall",         // 373
	"Ucircumflexsmall",    // 374
	"Udieresissmall",      // 375
	"Yacutesmall",         // 376
	"Thornsmall",          // 377
	"Ydieresissmall",      // 378
	"001.000",             // 379
	"001.001",             // 380
	"001.002",             // 381
	"001.003",             // 382
	"Black",               // 383
	"Bold",                // 384
	"Book",                // 385
	"Light",               // 386
	"Medium",              // 387
	"Regular",             // 388
	"Roman",               // 389
	"Semibold",            // 390
}

var nStdString = sid(len(stdStrings))
