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

var zapfDingbatsEncodingHas = map[string]bool{
	"a1":    true,
	"a10":   true,
	"a100":  true,
	"a101":  true,
	"a102":  true,
	"a103":  true,
	"a104":  true,
	"a105":  true,
	"a106":  true,
	"a107":  true,
	"a108":  true,
	"a109":  true,
	"a11":   true,
	"a110":  true,
	"a111":  true,
	"a112":  true,
	"a117":  true,
	"a118":  true,
	"a119":  true,
	"a12":   true,
	"a120":  true,
	"a121":  true,
	"a122":  true,
	"a123":  true,
	"a124":  true,
	"a125":  true,
	"a126":  true,
	"a127":  true,
	"a128":  true,
	"a129":  true,
	"a13":   true,
	"a130":  true,
	"a131":  true,
	"a132":  true,
	"a133":  true,
	"a134":  true,
	"a135":  true,
	"a136":  true,
	"a137":  true,
	"a138":  true,
	"a139":  true,
	"a14":   true,
	"a140":  true,
	"a141":  true,
	"a142":  true,
	"a143":  true,
	"a144":  true,
	"a145":  true,
	"a146":  true,
	"a147":  true,
	"a148":  true,
	"a149":  true,
	"a15":   true,
	"a150":  true,
	"a151":  true,
	"a152":  true,
	"a153":  true,
	"a154":  true,
	"a155":  true,
	"a156":  true,
	"a157":  true,
	"a158":  true,
	"a159":  true,
	"a16":   true,
	"a160":  true,
	"a161":  true,
	"a162":  true,
	"a163":  true,
	"a164":  true,
	"a165":  true,
	"a166":  true,
	"a167":  true,
	"a168":  true,
	"a169":  true,
	"a17":   true,
	"a170":  true,
	"a171":  true,
	"a172":  true,
	"a173":  true,
	"a174":  true,
	"a175":  true,
	"a176":  true,
	"a177":  true,
	"a178":  true,
	"a179":  true,
	"a18":   true,
	"a180":  true,
	"a181":  true,
	"a182":  true,
	"a183":  true,
	"a184":  true,
	"a185":  true,
	"a186":  true,
	"a187":  true,
	"a188":  true,
	"a189":  true,
	"a19":   true,
	"a190":  true,
	"a191":  true,
	"a192":  true,
	"a193":  true,
	"a194":  true,
	"a195":  true,
	"a196":  true,
	"a197":  true,
	"a198":  true,
	"a199":  true,
	"a2":    true,
	"a20":   true,
	"a200":  true,
	"a201":  true,
	"a202":  true,
	"a203":  true,
	"a204":  true,
	"a21":   true,
	"a22":   true,
	"a23":   true,
	"a24":   true,
	"a25":   true,
	"a26":   true,
	"a27":   true,
	"a28":   true,
	"a29":   true,
	"a3":    true,
	"a30":   true,
	"a31":   true,
	"a32":   true,
	"a33":   true,
	"a34":   true,
	"a35":   true,
	"a36":   true,
	"a37":   true,
	"a38":   true,
	"a39":   true,
	"a4":    true,
	"a40":   true,
	"a41":   true,
	"a42":   true,
	"a43":   true,
	"a44":   true,
	"a45":   true,
	"a46":   true,
	"a47":   true,
	"a48":   true,
	"a49":   true,
	"a5":    true,
	"a50":   true,
	"a51":   true,
	"a52":   true,
	"a53":   true,
	"a54":   true,
	"a55":   true,
	"a56":   true,
	"a57":   true,
	"a58":   true,
	"a59":   true,
	"a6":    true,
	"a60":   true,
	"a61":   true,
	"a62":   true,
	"a63":   true,
	"a64":   true,
	"a65":   true,
	"a66":   true,
	"a67":   true,
	"a68":   true,
	"a69":   true,
	"a7":    true,
	"a70":   true,
	"a71":   true,
	"a72":   true,
	"a73":   true,
	"a74":   true,
	"a75":   true,
	"a76":   true,
	"a77":   true,
	"a78":   true,
	"a79":   true,
	"a8":    true,
	"a81":   true,
	"a82":   true,
	"a83":   true,
	"a84":   true,
	"a9":    true,
	"a97":   true,
	"a98":   true,
	"a99":   true,
	"space": true,
}

var zapfDingbatsEncoding = [256]string{
	".notdef", // 0o000 = 0
	".notdef", // 0o001 = 1
	".notdef", // 0o002 = 2
	".notdef", // 0o003 = 3
	".notdef", // 0o004 = 4
	".notdef", // 0o005 = 5
	".notdef", // 0o006 = 6
	".notdef", // 0o007 = 7
	".notdef", // 0o010 = 8
	".notdef", // 0o011 = 9
	".notdef", // 0o012 = 10
	".notdef", // 0o013 = 11
	".notdef", // 0o014 = 12
	".notdef", // 0o015 = 13
	".notdef", // 0o016 = 14
	".notdef", // 0o017 = 15
	".notdef", // 0o020 = 16
	".notdef", // 0o021 = 17
	".notdef", // 0o022 = 18
	".notdef", // 0o023 = 19
	".notdef", // 0o024 = 20
	".notdef", // 0o025 = 21
	".notdef", // 0o026 = 22
	".notdef", // 0o027 = 23
	".notdef", // 0o030 = 24
	".notdef", // 0o031 = 25
	".notdef", // 0o032 = 26
	".notdef", // 0o033 = 27
	".notdef", // 0o034 = 28
	".notdef", // 0o035 = 29
	".notdef", // 0o036 = 30
	".notdef", // 0o037 = 31
	"space",   // 0o040 = 32
	"a1",      // 0o041 = 33
	"a2",      // 0o042 = 34
	"a202",    // 0o043 = 35
	"a3",      // 0o044 = 36
	"a4",      // 0o045 = 37
	"a5",      // 0o046 = 38
	"a119",    // 0o047 = 39
	"a118",    // 0o050 = 40
	"a117",    // 0o051 = 41
	"a11",     // 0o052 = 42
	"a12",     // 0o053 = 43
	"a13",     // 0o054 = 44
	"a14",     // 0o055 = 45
	"a15",     // 0o056 = 46
	"a16",     // 0o057 = 47
	"a105",    // 0o060 = 48
	"a17",     // 0o061 = 49
	"a18",     // 0o062 = 50
	"a19",     // 0o063 = 51
	"a20",     // 0o064 = 52
	"a21",     // 0o065 = 53
	"a22",     // 0o066 = 54
	"a23",     // 0o067 = 55
	"a24",     // 0o070 = 56
	"a25",     // 0o071 = 57
	"a26",     // 0o072 = 58
	"a27",     // 0o073 = 59
	"a28",     // 0o074 = 60
	"a6",      // 0o075 = 61
	"a7",      // 0o076 = 62
	"a8",      // 0o077 = 63
	"a9",      // 0o100 = 64
	"a10",     // 0o101 = 65
	"a29",     // 0o102 = 66
	"a30",     // 0o103 = 67
	"a31",     // 0o104 = 68
	"a32",     // 0o105 = 69
	"a33",     // 0o106 = 70
	"a34",     // 0o107 = 71
	"a35",     // 0o110 = 72
	"a36",     // 0o111 = 73
	"a37",     // 0o112 = 74
	"a38",     // 0o113 = 75
	"a39",     // 0o114 = 76
	"a40",     // 0o115 = 77
	"a41",     // 0o116 = 78
	"a42",     // 0o117 = 79
	"a43",     // 0o120 = 80
	"a44",     // 0o121 = 81
	"a45",     // 0o122 = 82
	"a46",     // 0o123 = 83
	"a47",     // 0o124 = 84
	"a48",     // 0o125 = 85
	"a49",     // 0o126 = 86
	"a50",     // 0o127 = 87
	"a51",     // 0o130 = 88
	"a52",     // 0o131 = 89
	"a53",     // 0o132 = 90
	"a54",     // 0o133 = 91
	"a55",     // 0o134 = 92
	"a56",     // 0o135 = 93
	"a57",     // 0o136 = 94
	"a58",     // 0o137 = 95
	"a59",     // 0o140 = 96
	"a60",     // 0o141 = 97
	"a61",     // 0o142 = 98
	"a62",     // 0o143 = 99
	"a63",     // 0o144 = 100
	"a64",     // 0o145 = 101
	"a65",     // 0o146 = 102
	"a66",     // 0o147 = 103
	"a67",     // 0o150 = 104
	"a68",     // 0o151 = 105
	"a69",     // 0o152 = 106
	"a70",     // 0o153 = 107
	"a71",     // 0o154 = 108
	"a72",     // 0o155 = 109
	"a73",     // 0o156 = 110
	"a74",     // 0o157 = 111
	"a203",    // 0o160 = 112
	"a75",     // 0o161 = 113
	"a204",    // 0o162 = 114
	"a76",     // 0o163 = 115
	"a77",     // 0o164 = 116
	"a78",     // 0o165 = 117
	"a79",     // 0o166 = 118
	"a81",     // 0o167 = 119
	"a82",     // 0o170 = 120
	"a83",     // 0o171 = 121
	"a84",     // 0o172 = 122
	"a97",     // 0o173 = 123
	"a98",     // 0o174 = 124
	"a99",     // 0o175 = 125
	"a100",    // 0o176 = 126
	".notdef", // 0o177 = 127
	".notdef", // 0o200 = 128
	".notdef", // 0o201 = 129
	".notdef", // 0o202 = 130
	".notdef", // 0o203 = 131
	".notdef", // 0o204 = 132
	".notdef", // 0o205 = 133
	".notdef", // 0o206 = 134
	".notdef", // 0o207 = 135
	".notdef", // 0o210 = 136
	".notdef", // 0o211 = 137
	".notdef", // 0o212 = 138
	".notdef", // 0o213 = 139
	".notdef", // 0o214 = 140
	".notdef", // 0o215 = 141
	".notdef", // 0o216 = 142
	".notdef", // 0o217 = 143
	".notdef", // 0o220 = 144
	".notdef", // 0o221 = 145
	".notdef", // 0o222 = 146
	".notdef", // 0o223 = 147
	".notdef", // 0o224 = 148
	".notdef", // 0o225 = 149
	".notdef", // 0o226 = 150
	".notdef", // 0o227 = 151
	".notdef", // 0o230 = 152
	".notdef", // 0o231 = 153
	".notdef", // 0o232 = 154
	".notdef", // 0o233 = 155
	".notdef", // 0o234 = 156
	".notdef", // 0o235 = 157
	".notdef", // 0o236 = 158
	".notdef", // 0o237 = 159
	".notdef", // 0o240 = 160
	"a101",    // 0o241 = 161
	"a102",    // 0o242 = 162
	"a103",    // 0o243 = 163
	"a104",    // 0o244 = 164
	"a106",    // 0o245 = 165
	"a107",    // 0o246 = 166
	"a108",    // 0o247 = 167
	"a112",    // 0o250 = 168
	"a111",    // 0o251 = 169
	"a110",    // 0o252 = 170
	"a109",    // 0o253 = 171
	"a120",    // 0o254 = 172
	"a121",    // 0o255 = 173
	"a122",    // 0o256 = 174
	"a123",    // 0o257 = 175
	"a124",    // 0o260 = 176
	"a125",    // 0o261 = 177
	"a126",    // 0o262 = 178
	"a127",    // 0o263 = 179
	"a128",    // 0o264 = 180
	"a129",    // 0o265 = 181
	"a130",    // 0o266 = 182
	"a131",    // 0o267 = 183
	"a132",    // 0o270 = 184
	"a133",    // 0o271 = 185
	"a134",    // 0o272 = 186
	"a135",    // 0o273 = 187
	"a136",    // 0o274 = 188
	"a137",    // 0o275 = 189
	"a138",    // 0o276 = 190
	"a139",    // 0o277 = 191
	"a140",    // 0o300 = 192
	"a141",    // 0o301 = 193
	"a142",    // 0o302 = 194
	"a143",    // 0o303 = 195
	"a144",    // 0o304 = 196
	"a145",    // 0o305 = 197
	"a146",    // 0o306 = 198
	"a147",    // 0o307 = 199
	"a148",    // 0o310 = 200
	"a149",    // 0o311 = 201
	"a150",    // 0o312 = 202
	"a151",    // 0o313 = 203
	"a152",    // 0o314 = 204
	"a153",    // 0o315 = 205
	"a154",    // 0o316 = 206
	"a155",    // 0o317 = 207
	"a156",    // 0o320 = 208
	"a157",    // 0o321 = 209
	"a158",    // 0o322 = 210
	"a159",    // 0o323 = 211
	"a160",    // 0o324 = 212
	"a161",    // 0o325 = 213
	"a163",    // 0o326 = 214
	"a164",    // 0o327 = 215
	"a196",    // 0o330 = 216
	"a165",    // 0o331 = 217
	"a192",    // 0o332 = 218
	"a166",    // 0o333 = 219
	"a167",    // 0o334 = 220
	"a168",    // 0o335 = 221
	"a169",    // 0o336 = 222
	"a170",    // 0o337 = 223
	"a171",    // 0o340 = 224
	"a172",    // 0o341 = 225
	"a173",    // 0o342 = 226
	"a162",    // 0o343 = 227
	"a174",    // 0o344 = 228
	"a175",    // 0o345 = 229
	"a176",    // 0o346 = 230
	"a177",    // 0o347 = 231
	"a178",    // 0o350 = 232
	"a179",    // 0o351 = 233
	"a193",    // 0o352 = 234
	"a180",    // 0o353 = 235
	"a199",    // 0o354 = 236
	"a181",    // 0o355 = 237
	"a200",    // 0o356 = 238
	"a182",    // 0o357 = 239
	".notdef", // 0o360 = 240
	"a201",    // 0o361 = 241
	"a183",    // 0o362 = 242
	"a184",    // 0o363 = 243
	"a197",    // 0o364 = 244
	"a185",    // 0o365 = 245
	"a194",    // 0o366 = 246
	"a198",    // 0o367 = 247
	"a186",    // 0o370 = 248
	"a195",    // 0o371 = 249
	"a187",    // 0o372 = 250
	"a188",    // 0o373 = 251
	"a189",    // 0o374 = 252
	"a190",    // 0o375 = 253
	"a191",    // 0o376 = 254
	".notdef", // 0o377 = 255
}
