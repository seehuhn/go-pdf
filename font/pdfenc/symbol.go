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

package pdfenc

// IsSymbol is the character set of the Symbol font.
var IsSymbol = map[string]bool{
	"Alpha":          true,
	"Beta":           true,
	"Chi":            true,
	"Delta":          true,
	"Epsilon":        true,
	"Eta":            true,
	"Euro":           true,
	"Gamma":          true,
	"Ifraktur":       true,
	"Iota":           true,
	"Kappa":          true,
	"Lambda":         true,
	"Mu":             true,
	"Nu":             true,
	"Omega":          true,
	"Omicron":        true,
	"Phi":            true,
	"Pi":             true,
	"Psi":            true,
	"Rfraktur":       true,
	"Rho":            true,
	"Sigma":          true,
	"Tau":            true,
	"Theta":          true,
	"Upsilon":        true,
	"Upsilon1":       true,
	"Xi":             true,
	"Zeta":           true,
	"aleph":          true,
	"alpha":          true,
	"ampersand":      true,
	"angle":          true,
	"angleleft":      true,
	"angleright":     true,
	"approxequal":    true,
	"arrowboth":      true,
	"arrowdblboth":   true,
	"arrowdbldown":   true,
	"arrowdblleft":   true,
	"arrowdblright":  true,
	"arrowdblup":     true,
	"arrowdown":      true,
	"arrowhorizex":   true,
	"arrowleft":      true,
	"arrowright":     true,
	"arrowup":        true,
	"arrowvertex":    true,
	"asteriskmath":   true,
	"bar":            true,
	"beta":           true,
	"braceex":        true,
	"braceleft":      true,
	"braceleftbt":    true,
	"braceleftmid":   true,
	"bracelefttp":    true,
	"braceright":     true,
	"bracerightbt":   true,
	"bracerightmid":  true,
	"bracerighttp":   true,
	"bracketleft":    true,
	"bracketleftbt":  true,
	"bracketleftex":  true,
	"bracketlefttp":  true,
	"bracketright":   true,
	"bracketrightbt": true,
	"bracketrightex": true,
	"bracketrighttp": true,
	"bullet":         true,
	"carriagereturn": true,
	"chi":            true,
	"circlemultiply": true,
	"circleplus":     true,
	"club":           true,
	"colon":          true,
	"comma":          true,
	"congruent":      true,
	"copyrightsans":  true,
	"copyrightserif": true,
	"degree":         true,
	"delta":          true,
	"diamond":        true,
	"divide":         true,
	"dotmath":        true,
	"eight":          true,
	"element":        true,
	"ellipsis":       true,
	"emptyset":       true,
	"epsilon":        true,
	"equal":          true,
	"equivalence":    true,
	"eta":            true,
	"exclam":         true,
	"existential":    true,
	"five":           true,
	"florin":         true,
	"four":           true,
	"fraction":       true,
	"gamma":          true,
	"gradient":       true,
	"greater":        true,
	"greaterequal":   true,
	"heart":          true,
	"infinity":       true,
	"integral":       true,
	"integralbt":     true,
	"integralex":     true,
	"integraltp":     true,
	"intersection":   true,
	"iota":           true,
	"kappa":          true,
	"lambda":         true,
	"less":           true,
	"lessequal":      true,
	"logicaland":     true,
	"logicalnot":     true,
	"logicalor":      true,
	"lozenge":        true,
	"minus":          true,
	"minute":         true,
	"mu":             true,
	"multiply":       true,
	"nine":           true,
	"notelement":     true,
	"notequal":       true,
	"notsubset":      true,
	"nu":             true,
	"numbersign":     true,
	"omega":          true,
	"omega1":         true,
	"omicron":        true,
	"one":            true,
	"parenleft":      true,
	"parenleftbt":    true,
	"parenleftex":    true,
	"parenlefttp":    true,
	"parenright":     true,
	"parenrightbt":   true,
	"parenrightex":   true,
	"parenrighttp":   true,
	"partialdiff":    true,
	"percent":        true,
	"period":         true,
	"perpendicular":  true,
	"phi":            true,
	"phi1":           true,
	"pi":             true,
	"plus":           true,
	"plusminus":      true,
	"product":        true,
	"propersubset":   true,
	"propersuperset": true,
	"proportional":   true,
	"psi":            true,
	"question":       true,
	"radical":        true,
	"radicalex":      true,
	"reflexsubset":   true,
	"reflexsuperset": true,
	"registersans":   true,
	"registerserif":  true,
	"rho":            true,
	"second":         true,
	"semicolon":      true,
	"seven":          true,
	"sigma":          true,
	"sigma1":         true,
	"similar":        true,
	"six":            true,
	"slash":          true,
	"space":          true,
	"spade":          true,
	"suchthat":       true,
	"summation":      true,
	"tau":            true,
	"therefore":      true,
	"theta":          true,
	"theta1":         true,
	"three":          true,
	"trademarksans":  true,
	"trademarkserif": true,
	"two":            true,
	"underscore":     true,
	"union":          true,
	"universal":      true,
	"upsilon":        true,
	"weierstrass":    true,
	"xi":             true,
	"zero":           true,
	"zeta":           true,
}

// SymbolEncoding is the built-in encoding for the Symbol font.
//
// See Appendix D.5 of PDF 32000-1:2008.
var SymbolEncoding = [256]string{
	".notdef",        // 0o000 = 0
	".notdef",        // 0o001 = 1
	".notdef",        // 0o002 = 2
	".notdef",        // 0o003 = 3
	".notdef",        // 0o004 = 4
	".notdef",        // 0o005 = 5
	".notdef",        // 0o006 = 6
	".notdef",        // 0o007 = 7
	".notdef",        // 0o010 = 8
	".notdef",        // 0o011 = 9
	".notdef",        // 0o012 = 10
	".notdef",        // 0o013 = 11
	".notdef",        // 0o014 = 12
	".notdef",        // 0o015 = 13
	".notdef",        // 0o016 = 14
	".notdef",        // 0o017 = 15
	".notdef",        // 0o020 = 16
	".notdef",        // 0o021 = 17
	".notdef",        // 0o022 = 18
	".notdef",        // 0o023 = 19
	".notdef",        // 0o024 = 20
	".notdef",        // 0o025 = 21
	".notdef",        // 0o026 = 22
	".notdef",        // 0o027 = 23
	".notdef",        // 0o030 = 24
	".notdef",        // 0o031 = 25
	".notdef",        // 0o032 = 26
	".notdef",        // 0o033 = 27
	".notdef",        // 0o034 = 28
	".notdef",        // 0o035 = 29
	".notdef",        // 0o036 = 30
	".notdef",        // 0o037 = 31
	"space",          // 0o040 = 32
	"exclam",         // 0o041 = 33
	"universal",      // 0o042 = 34
	"numbersign",     // 0o043 = 35
	"existential",    // 0o044 = 36
	"percent",        // 0o045 = 37
	"ampersand",      // 0o046 = 38
	"suchthat",       // 0o047 = 39
	"parenleft",      // 0o050 = 40
	"parenright",     // 0o051 = 41
	"asteriskmath",   // 0o052 = 42
	"plus",           // 0o053 = 43
	"comma",          // 0o054 = 44
	"minus",          // 0o055 = 45
	"period",         // 0o056 = 46
	"slash",          // 0o057 = 47
	"zero",           // 0o060 = 48
	"one",            // 0o061 = 49
	"two",            // 0o062 = 50
	"three",          // 0o063 = 51
	"four",           // 0o064 = 52
	"five",           // 0o065 = 53
	"six",            // 0o066 = 54
	"seven",          // 0o067 = 55
	"eight",          // 0o070 = 56
	"nine",           // 0o071 = 57
	"colon",          // 0o072 = 58
	"semicolon",      // 0o073 = 59
	"less",           // 0o074 = 60
	"equal",          // 0o075 = 61
	"greater",        // 0o076 = 62
	"question",       // 0o077 = 63
	"congruent",      // 0o100 = 64
	"Alpha",          // 0o101 = 65
	"Beta",           // 0o102 = 66
	"Chi",            // 0o103 = 67
	"Delta",          // 0o104 = 68
	"Epsilon",        // 0o105 = 69
	"Phi",            // 0o106 = 70
	"Gamma",          // 0o107 = 71
	"Eta",            // 0o110 = 72
	"Iota",           // 0o111 = 73
	"theta1",         // 0o112 = 74
	"Kappa",          // 0o113 = 75
	"Lambda",         // 0o114 = 76
	"Mu",             // 0o115 = 77
	"Nu",             // 0o116 = 78
	"Omicron",        // 0o117 = 79
	"Pi",             // 0o120 = 80
	"Theta",          // 0o121 = 81
	"Rho",            // 0o122 = 82
	"Sigma",          // 0o123 = 83
	"Tau",            // 0o124 = 84
	"Upsilon",        // 0o125 = 85
	"sigma1",         // 0o126 = 86
	"Omega",          // 0o127 = 87
	"Xi",             // 0o130 = 88
	"Psi",            // 0o131 = 89
	"Zeta",           // 0o132 = 90
	"bracketleft",    // 0o133 = 91
	"therefore",      // 0o134 = 92
	"bracketright",   // 0o135 = 93
	"perpendicular",  // 0o136 = 94
	"underscore",     // 0o137 = 95
	"radicalex",      // 0o140 = 96
	"alpha",          // 0o141 = 97
	"beta",           // 0o142 = 98
	"chi",            // 0o143 = 99
	"delta",          // 0o144 = 100
	"epsilon",        // 0o145 = 101
	"phi",            // 0o146 = 102
	"gamma",          // 0o147 = 103
	"eta",            // 0o150 = 104
	"iota",           // 0o151 = 105
	"phi1",           // 0o152 = 106
	"kappa",          // 0o153 = 107
	"lambda",         // 0o154 = 108
	"mu",             // 0o155 = 109
	"nu",             // 0o156 = 110
	"omicron",        // 0o157 = 111
	"pi",             // 0o160 = 112
	"theta",          // 0o161 = 113
	"rho",            // 0o162 = 114
	"sigma",          // 0o163 = 115
	"tau",            // 0o164 = 116
	"upsilon",        // 0o165 = 117
	"omega1",         // 0o166 = 118
	"omega",          // 0o167 = 119
	"xi",             // 0o170 = 120
	"psi",            // 0o171 = 121
	"zeta",           // 0o172 = 122
	"braceleft",      // 0o173 = 123
	"bar",            // 0o174 = 124
	"braceright",     // 0o175 = 125
	"similar",        // 0o176 = 126
	".notdef",        // 0o177 = 127
	".notdef",        // 0o200 = 128
	".notdef",        // 0o201 = 129
	".notdef",        // 0o202 = 130
	".notdef",        // 0o203 = 131
	".notdef",        // 0o204 = 132
	".notdef",        // 0o205 = 133
	".notdef",        // 0o206 = 134
	".notdef",        // 0o207 = 135
	".notdef",        // 0o210 = 136
	".notdef",        // 0o211 = 137
	".notdef",        // 0o212 = 138
	".notdef",        // 0o213 = 139
	".notdef",        // 0o214 = 140
	".notdef",        // 0o215 = 141
	".notdef",        // 0o216 = 142
	".notdef",        // 0o217 = 143
	".notdef",        // 0o220 = 144
	".notdef",        // 0o221 = 145
	".notdef",        // 0o222 = 146
	".notdef",        // 0o223 = 147
	".notdef",        // 0o224 = 148
	".notdef",        // 0o225 = 149
	".notdef",        // 0o226 = 150
	".notdef",        // 0o227 = 151
	".notdef",        // 0o230 = 152
	".notdef",        // 0o231 = 153
	".notdef",        // 0o232 = 154
	".notdef",        // 0o233 = 155
	".notdef",        // 0o234 = 156
	".notdef",        // 0o235 = 157
	".notdef",        // 0o236 = 158
	".notdef",        // 0o237 = 159
	"Euro",           // 0o240 = 160
	"Upsilon1",       // 0o241 = 161
	"minute",         // 0o242 = 162
	"lessequal",      // 0o243 = 163
	"fraction",       // 0o244 = 164
	"infinity",       // 0o245 = 165
	"florin",         // 0o246 = 166
	"club",           // 0o247 = 167
	"diamond",        // 0o250 = 168
	"heart",          // 0o251 = 169
	"spade",          // 0o252 = 170
	"arrowboth",      // 0o253 = 171
	"arrowleft",      // 0o254 = 172
	"arrowup",        // 0o255 = 173
	"arrowright",     // 0o256 = 174
	"arrowdown",      // 0o257 = 175
	"degree",         // 0o260 = 176
	"plusminus",      // 0o261 = 177
	"second",         // 0o262 = 178
	"greaterequal",   // 0o263 = 179
	"multiply",       // 0o264 = 180
	"proportional",   // 0o265 = 181
	"partialdiff",    // 0o266 = 182
	"bullet",         // 0o267 = 183
	"divide",         // 0o270 = 184
	"notequal",       // 0o271 = 185
	"equivalence",    // 0o272 = 186
	"approxequal",    // 0o273 = 187
	"ellipsis",       // 0o274 = 188
	"arrowvertex",    // 0o275 = 189
	"arrowhorizex",   // 0o276 = 190
	"carriagereturn", // 0o277 = 191
	"aleph",          // 0o300 = 192
	"Ifraktur",       // 0o301 = 193
	"Rfraktur",       // 0o302 = 194
	"weierstrass",    // 0o303 = 195
	"circlemultiply", // 0o304 = 196
	"circleplus",     // 0o305 = 197
	"emptyset",       // 0o306 = 198
	"intersection",   // 0o307 = 199
	"union",          // 0o310 = 200
	"propersuperset", // 0o311 = 201
	"reflexsuperset", // 0o312 = 202
	"notsubset",      // 0o313 = 203
	"propersubset",   // 0o314 = 204
	"reflexsubset",   // 0o315 = 205
	"element",        // 0o316 = 206
	"notelement",     // 0o317 = 207
	"angle",          // 0o320 = 208
	"gradient",       // 0o321 = 209
	"registerserif",  // 0o322 = 210
	"copyrightserif", // 0o323 = 211
	"trademarkserif", // 0o324 = 212
	"product",        // 0o325 = 213
	"radical",        // 0o326 = 214
	"dotmath",        // 0o327 = 215
	"logicalnot",     // 0o330 = 216
	"logicaland",     // 0o331 = 217
	"logicalor",      // 0o332 = 218
	"arrowdblboth",   // 0o333 = 219
	"arrowdblleft",   // 0o334 = 220
	"arrowdblup",     // 0o335 = 221
	"arrowdblright",  // 0o336 = 222
	"arrowdbldown",   // 0o337 = 223
	"lozenge",        // 0o340 = 224
	"angleleft",      // 0o341 = 225
	"registersans",   // 0o342 = 226
	"copyrightsans",  // 0o343 = 227
	"trademarksans",  // 0o344 = 228
	"summation",      // 0o345 = 229
	"parenlefttp",    // 0o346 = 230
	"parenleftex",    // 0o347 = 231
	"parenleftbt",    // 0o350 = 232
	"bracketlefttp",  // 0o351 = 233
	"bracketleftex",  // 0o352 = 234
	"bracketleftbt",  // 0o353 = 235
	"bracelefttp",    // 0o354 = 236
	"braceleftmid",   // 0o355 = 237
	"braceleftbt",    // 0o356 = 238
	"braceex",        // 0o357 = 239
	".notdef",        // 0o360 = 240
	"angleright",     // 0o361 = 241
	"integral",       // 0o362 = 242
	"integraltp",     // 0o363 = 243
	"integralex",     // 0o364 = 244
	"integralbt",     // 0o365 = 245
	"parenrighttp",   // 0o366 = 246
	"parenrightex",   // 0o367 = 247
	"parenrightbt",   // 0o370 = 248
	"bracketrighttp", // 0o371 = 249
	"bracketrightex", // 0o372 = 250
	"bracketrightbt", // 0o373 = 251
	"bracerighttp",   // 0o374 = 252
	"bracerightmid",  // 0o375 = 253
	"bracerightbt",   // 0o376 = 254
	".notdef",        // 0o377 = 255
}
