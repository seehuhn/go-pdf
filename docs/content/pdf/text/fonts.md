+++
title = 'PDF Font Types'
date = 2024-09-18T11:13:08+01:00
weight = 10
+++


# PDF Font Types

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

An alternative way to present this information is given in the following table.
The table shows which types of font data are allowed in which type of font
dictionary.

|                   | Type 1 | CFF, OpenType/CFF | TrueType, OpenType/glyf | Type 3 |
|-------------------|:------:|:-----------------:|:-----------------------:|:------:|
| `Type1`/`MMType1` | X      | X                 |                         |        |
| `CIDFontType0`    |        | X                 |                         |        |
| `TrueType`        |        |                   | X                       |        |
| `CIDFontType2`    |        |                   | X                       |        |
| `Type3`           |        |                   |                         | X      |

The following sections summarise how font information is structured in
PDF files.

## All Font Types

The following information applies to all font PDF fonts.

- Every font is described by a font dictionary.  The `Type` entry of these
  dictionaries is `Font`, the `Subtype` entry is used to distinguish between
  different font types.

- For all fonts, the optional `ToUnicode` entry in the font dictionary can be
  used to map character codes to Unicode code points, for text extraction.


## Simple Fonts

Simple fonts use single byte character codes which map to glyph names.
Glyph names are then mapped to glyphs in the font file.
The following information applies to simple PDF fonts.

- Using information from the `Encoding` entry in the font dictionary, character
  codes are mapped to glyph names, and glyph names are mapped to glyphs.

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

- Glyphs can be selected by glyph name or using the built-in encoding.

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
    set to `Type1C`.  Glyph names are required, and thus fonts which use CIDFont
    operators in their Top DICT cannot be used.

  - OpenType fonts are embedded using the `FontFile3` entry in the font
    descriptor, and the `Subtype` entry in the font dictionary is set to
    `OpenType`.  The CFF font is not allowed to use CIDFont operators.
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

### CFF and OpenType with CFF glyph outlines

- The `Subtype` in the CIDFont dictionary is `CIDFontType0`.

- The PostScript name of the font is given by the `BaseFont` entry in the
  CIDFont dictionary and the `FontName` entry in the font descriptor.  The name
  may be prefixed by a subset tag (e.g. `ABCDEF+Times-Roman`).

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

- Embedding:
  - TrueType fonts are embedded using the `FontFile2` entry in the font
    descriptor.
  - OpenType fonts are embedded using the `FontFile3` entry in the font
    descriptor, and the `Subtype` entry in the font dictionary is set to
    `OpenType`.

  The following tables are required: "glyf", "head", "hhea", "hmtx", "loca",
  "maxp". If used by the font, "cvt ", "fpgm", "prep" are also required.
