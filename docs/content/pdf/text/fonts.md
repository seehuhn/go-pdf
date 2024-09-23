+++
title = 'Font Summary'
date = 2024-09-18T11:13:08+01:00
draft = true
+++


# Font Summary

The table lists all font types allowed in PDF 2.0.

|   | Font Dict   | CIDFont Dict   | FontDescriptor | Stream Dict     | Description     |
|---| ----------- | -------------- | -------------- | --------------- | --------------- |
| 1 | `Type1`     | -              | `FontFile`     |                 | Type 1          |
|   | `Type1`     | -              | `FontFile3`    | `Type1C`        | CFF             |
|   | `Type1`     | -              | `FontFile3`    | `OpenType`      | OpenType/CFF    |
|   | `Type1`     | -              |                | -               | external Type 1 |
|   | `MMType1`   | -              | `FontFile`     |                 | Type 1          |
|   | `MMType1`   | -              | `FontFile3`    | `Type1C`        | CFF             |
|   | `MMType1`   | -              |                | -               | ext. MMType1    |
| 2 | `TrueType`  | -              | `FontFile2`    |                 | TrueType        |
|   | `TrueType`  | -              | `FontFile3`    | `OpenType`      | OpenType/glyf   |
|   | `TrueType`  | -              |                | -               | ext. TrueType   |
| 3 | `Type3`     | -              | -              | -               | Type 3          |
| 4 | `Type0`     | `CIDFontType0` | `FontFile3`    | `CIDFontType0C` | CFF             |
|   | `Type0`     | `CIDFontType0` | `FontFile3`    | `OpenType`      | OpenType/CFF    |
|   | `Type0`     | `CIDFontType0` |                | -               | external CFF    |
| 5 | `Type0`     | `CIDFontType2` | `FontFile2`    |                 | TrueType        |
|   | `Type0`     | `CIDFontType2` | `FontFile3`    | `OpenType`      | OpenType/glyf   |
|   | `Type0`     | `CIDFontType2` |                | -               | ext. TrueType   |

The columns contain the following information:
- The `Subtype` entry in the font dictionary.
- The `Subtype` entry in the CIDFont dictionary (for composite fonts).
- Name of the font descriptor entry pointing to the font data stream (for embedded fonts).
- The `Subtype` entry in the font data stream dictionary, if any.
- A short description of the font type.

The following sections summarise how font information is structured in
PDF files.  The information is based on the PDF 2.0 specification.

## All Font Types

The following information applies to all font PDF fonts.

- Every font is described by a font dictionary.  The `Type` entry of these
  dictionaries is `Font`, the `Subtype` entry is used to distinguish between
  different font types.

- For all fonts, the `ToUnicode` entry in the font dictionary (optional) maps
  character codes to Unicode code points.

- Except for Type 3 fonts, the glyph data can either be embedded in the PDF
  file, or be loaded from an external file by the viewer. Unless text rendering
  mode 3 (invisible) is used, it is recommended to embed the font.


## Simple Fonts

Simple fonts use single byte character codes which map to glyph names.
Glyph names are then mapped to glyphs in the font file.
The following information applies to simple PDF fonts.

- Using information from the `Encoding` entry in the font dictionary, character
  codes are mapped to glyph names, and glyph names are mapped to glyphs in the
  font file using the rules outlined below.

- If a glyph is not present in the font, the `.notdef` glyph is used instead.

- Simple fonts only support horizontal writing. Glyph widths are described by
  the `Widths` entry in the font dictionary and the `MissingWidth` entry of the
  font descriptor.  The widths array is indexed by character code.

- If no ToUnicode map is present, glyph names can be mapped to Unicode values
  using the Adobe Glyph List.

### Type 1, CFF, OpenType with CFF glyph outlines, and Multiple Master Type 1

- The `Subtype` value in the font dictionary is `Type1`, or `MMType1` for multiple
  master fonts.

- The PostScript name of the font is given as the `BaseFont` entry in the
  font dictionary and the `FontName` entry in the font descriptor.  If the font
  is embedded in the PDF file, the name may be prefixed by a subset tag (e.g.
  `ABCDEF+Times-Roman`).

  For `MMType1` fonts, space characters in the font name are replaced with
  underscore characters.

- The `Encoding` entry in the font dictionary describes how character codes
  are mapped to glyph names:
  - If `Encoding` is not set, the font's built-in encoding is used.
  - If `Encoding` is one of `MacRomanEncoding`, `MacExpertEncoding`, or
    `WinAnsiEncoding`, the corresponding encoding from appendix D of the spec
    is used.
  - If `Encoding` is a PDF dictionary, first construct a "base encoding":
    - If `BaseEncoding` is one of `MacRomanEncoding`, `MacExpertEncoding`, or
      `WinAnsiEncoding`, the corresponding encoding is used as the base
      encoding.
    - Otherwise, if the font is not embedded and non-symbolic,
      `StandardEncoding` is used.
    - Otherwise, the font's built-in encoding is used.

    Then, the updates described by the `Differences` array are applied to the
    base encoding to get the final encoding.

- Glyphs are selected by glyph name, using information stored in the font
  data.

- Embedding:
  - If original Type 1 format is used, the font is embedded using the `FontFile`
    entry in the font descriptor.  The `length1` entry in the stream dictionary
    gives the length of the clear-text portion of the font data.  The `length2`
    entry gives the length of the "encrypted" portion of the font data (which
    must use the binary format). The `length3` entry gives the length of the
    trailing zeros section of the font data, or is set to zero if trailing
    zeros are not included.

  - If the CFF format is used, the font is embedded using the `FontFile3` entry
    in the font descriptor, and the `Subtype` entry in the font dictionary is
    set to `Type1C`.  Only fonts which do not use CIDFont operators in their
    Top DICT are allowed, because Glyph names are required.

  - OpenType fonts are embedded using the `FontFile3` entry in the font
    descriptor, and the `Subtype` entry in the font dictionary is set to
    `OpenType`.  The CFF fonts data is not allowed to use CIDFont operators.
    The following OpenType tables are required: "CFF " and "cmap".

- `MMType1` fonts are embedded as ordinary Type 1 fonts (a snapshot of the
  multiple master font).  In case the font is loaded from an external file, the
  space/underscore-separated numbers in the font name give the values for the
  design axes.

### TrueType and OpenType with "glyf" outlines

- The `Subtype` value in the font dictionary is `TrueType`.

- The font name is given by the `BaseFont` entry in the font dictionary and the
  `FontName` entry in the font descriptor.  The name may be prefixed by a
  subset tag (e.g. `ABCDEF+Times-Roman`). The font name may either represent
  the PostScript name of the font (if present in the TrueType `"name"`
  table), or it may be "derived from the name by which the font is known in the
  host operating system".

- There are some restrictions on the `Encoding` entry in the font
  dictionary:
  - Sometimes between PDF-1.4 and PDF-1.7, use of of `MacExpertEncoding`
    was disallowed or at least discouraged.
  - Use of the `Differences` array is discouraged.

  In addition, after the table is constructed, any undefined entries in the
  table are filled using the Standard Encoding.

- The spec describes a variety of mechanisms to map a single-byte code `c` to a
  glyph in a TrueType font:
  1. Use a (1,0) "cmap" subtable to map `c` to a GID.
  2. In a (3,0) "cmap" subtable, look up either `c`, `c+0xF000`, `c+0xF100`,
     or `c+0xF200` to get a GID.
  3. Use the encoding to map `c` to a name, use Mac OS Roman to map the name to
     a code, and use a (1,0) "cmap" subtable to map this code to a GID.
  4. Use the encoding to map `c` to a name, use the Adobe Glyph List to map the
     name to unicode, and use a (3,1) "cmap" subtable to map this character to a
     GID.
  5. Use the encoding to map `c` to a name, and use the "post" table to look
     up the glyph.

  Because of ambiguities, the spec recommends to use composite fonts with
  `Encoding` set to `Identity-H` and `CIDToGIDMap` set to `Identity`, in place
  of simple fonts with TrueType outlines.

  I plan to implement the following behaviour: In each of the four cases, try
  the following methods and use the first one that succeeds:

  |             | non-symbolic | symbolic   |
  | ----------: | :----------: | :--------: |
  | encoding    | 4, 3         | 4, 2, 5, 1 |
  | no encoding | 2, 1         | 2, 1       |

- Embedding:
  - TrueType fonts are embedded using the `FontFile2` entry in the font
    descriptor.  The `length1` entry in the stream dictionary gives the length
    of the decoded font data.
  - OpenType fonts are embedded using the `FontFile3` entry in the font
    descriptor, and the `Subtype` entry in the font dictionary is set to
    `OpenType`.
  - Font programs "should" be embedded.

  The following tables are required: "glyf", "head", "hhea", "hmtx", "loca",
  "maxp", and either "cmap" or "post".  If used by the font, "cvt ", "fpgm",
  "prep" are also required.

### Type 3

- The `Subtype` value in the font dictionary is `Type3`.

- The font name is optional.  If it is present, it is given by the `Name` entry
  in the Type3 font dictionary and the `FontName` entry in the font descriptor.

- The `Encoding` entry in the font dictionary maps character codes to glyph
  names.  The complete encoding is given by the `Differences` entry in
  `Encoding` dict.

- The glyph descriptions are given by the `CharProcs` entry in the font
  dictionary.  The `CharProcs` entry is a dictionary glyph names to content
  streams.

- There is no default glyph for missing glyphs.
  The behaviour for missing glyphs is not specified.


## Composite Fonts

Composite fonts use multi-byte character codes, which map to numeric character
identifiers (CIDs).  CIDs are then mapped to glyphs in the font file.
The following information applies to composite PDF fonts.

- In addition to the font dictionary, composite fonts have a separate CIDFont dictionary.
  Information which requires knowledge about the
  encoding is in the the font dictionary, the remaining
  information is in the CIDFont dictionary.  The CIDFont dictionary can
  be located via the `DescendantFonts` entry in the font dictionary.

- The `Subtype` value in the font dictionary is `Type0`.  Different types of
  composite fonts are distinguished by the `Subtype` entry in the CIDFont
  dictionary.

- The `Encoding` entry in the font dictionary specifies a CMap which
  defines which byte sequences form valid character codes, and which
  CID values these character codes map to.

  If the byte sequence does not start with a valid character code, the
  following things happen:

  1. The longes prefix of the byte sequence which forms the start of a valid
     character code is identified. The number of input bytes consumed is the
     length of the shortest valid code which starts with this prefix.  In
     particular, if no valid code starts with the first byte of the given code,
     the number of bytes consumed is the length of the shortest code overall.
  2. CID 0 is used.

  If a valid code is found, the following things happen:

  1. If there is a `cidchar` or `cidrange` mapping for the code,
     the resulting CID is used.
  2. Otherwise, if there is a `notdef` mapping for the code,
     the resulting CID is used.
  3. Otherwise, CID 0 is used.

- CIDs are mapped to glyphs as explained below for the different font types.
  If the font does not contain a glyph for a CID, the following things happen:

  1. If the CMap contains a `notdef` mapping for the corresponding code, and
     there is a glyph for the CID from the `notdef` mapping, this glyph is shown.
  2. Otherwise, the glyph for CID 0 is shown.

- The `WMode` entry of the CMap specifies the writing mode (horizontal or
  vertical).

- Glyphs widths are described by the `W`, `DW`, `W2`, and `DW2` entries in the
  CIDFont dictionary.
  - For horizontal writing, only `W` and `DW` are used.
    `W` specifies the advance widths for some CIDs, and `DW` is used for
    all other CIDs.
  - For vertical writing, the entries encode a vertical advance distance, and
    an offset vector for all CIDs. For some CIDs, the three values are given in
    the `W2` array. For all other CIDs, the vertical values are given in the
    `DW2` array, and half of the horizontal advance width is used for the
    horizontal component of the offset vector.

- The `CIDSystemInfo` entry in the CIDFont dictionary and in the CMap specifies
  a "character collection". A character collection maps CID values to
  characters.  If a standard character collection is used, CID values
  can be mapped to Unicode values.

### CFF and OpenType with CFF glyph outlines

- The `Subtype` in the CIDFont dictionary is `CIDFontType0`.

- The PostScript name of the font is given by the `BaseFont` entry in the
  CIDFont dictionary and the `FontName` entry in the font descriptor.  The name
  may be prefixed by a subset tag (e.g. `ABCDEF+Times-Roman`).

- The mapping of CIDs to glyphs depends on the CFF font variant:
  - Some CFF fonts use "CIDFont operators" to map CIDs to glyphs.
    If such a mapping is present, it is used.
  - If the CFF font does not use CIDFont operators, the CID is used as the GID.

- Embedding:
  - CFF font data is embedded using the `FontFile3` entry in the font
    descriptor, and the `Subtype` entry in the font dictionary is set to
    `CIDFontType0C`.
  - OpenType fonts are embedded using the `FontFile3` entry in the font
    descriptor, and the `Subtype` entry in the font dictionary is set to
    `OpenType`.  The following tables are required: "CFF ", "cmap".

### TrueType and OpenType with "glyf" glyph outlines

- The `Subtype` in the CIDFont dictionary is `CIDFontType2`.

- The name is given in the `BaseFont` entry in the CIDFont dictionary and the
  `FontName` entry in the font descriptor. The name may be prefixed by a subset
  tag (e.g. `ABCDEF+Times-Roman`). The given name is either the PostScript name
  of the font (if present in the TrueType `"name"` table), or it is "derived
  from the name by which the font is known in the host operating system".

- Glyph selection depends on whether the font is embedded:
  - If the font is embedded, the `CIDToGIDMap` entry in the CIDFont dictionary
    is used to map CIDs to GIDs.
  - If the font is not embedded, CID values are mapped to unicode values, and
    the font's "cmap" table is used to map unicode values to glyphs. The spec
    requires that one of the pre-defined CMaps must be used in this case.

- Embedding:
  - TrueType fonts are embedded using the `FontFile2` entry in the font
    descriptor.
  - OpenType fonts are embedded using the `FontFile3` entry in the font
    descriptor, and the `Subtype` entry in the font dictionary is set to
    `OpenType`.

  The following tables are required: "glyf", "head", "hhea", "hmtx", "loca",
  "maxp". If used by the font, "cvt ", "fpgm", "prep" are also required.
