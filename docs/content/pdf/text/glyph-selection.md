+++
title = 'Glyph Selection'
date = 2025-05-23T10:53:17+01:00
+++

# Glyph Selection

This page lists all the different ways to select glyphs from a font file
in PDF.  There are many combinations of font dictionaries and font types.

## Overview

Font file types:
- Type1
- TrueType
- CFF with glyph names
- CFF with CIDs
- OpenType/glyf
- OpenType/CFF with glyph names
- OpenType/CFF with CIDs

Font dictionary types:
- Type1/MMType1. This supports the following font file types:
    - Type1
    - CFF with glyph names
    - OpenType/CFF with glyph names

  Glyphs are selected by name or via the built-in encoding.

- TrueType. This supports the following font file types:
    - TrueType
    - OpenType/glyf

  Glyphs are normally selected using a "built-in encoding", which itself
  is contained in one of the TrueType cmap subtables.  Glyphs can also
  be selected by name,

- CIDFontType0.  This supports the following font file types:
    - CFF with glyph names
    - CFF with CIDs
    - OpenType/CFF with glyph names
    - OpenType/CFF with CIDs

  Glyphs are selected by CID.  For fonts with glyph names, the CID is interpreted
  as the GID.

- CIDFontType2.  This dictionary type supports the following font file types:
    - TrueType
    - OpenType/glyf

  For embedded fonts, the CIDToGIDMap from the font dictionary is used to map
  CIDs to GIDs, which then are used to select the glyphs.  For external fonts,
  the text content implied by the CIDs is used.  The text content is looked up
  in the font file's "cmap" table to locate the appropriate glyph.

- Type3: Glyph data is contained in PDF content streams.  There is no separate
  font file.

## TrueType Fonts

TrueType fonts don't have a built-in encoding, and normally don't use glyph
names.  Instead, the spec describes a variety of mechanisms to map a
single-byte code `c` to a glyph in a TrueType font. An overview is given below.
For full details see section 9.6.5.4 (Encodings for TrueType fonts) of ISO
32000-2:2020.

1. Use a (1,0) "cmap" subtable to map `c` to a GID.
2. In a (3,0) "cmap" subtable, look up `c`, `c+0xF000`, `c+0xF100`, or
   `c+0xF200`, in order, to get a GID.
3. Use the encoding to map `c` to a name, use Mac OS Roman to map the name to
    a code, and use a (1,0) "cmap" subtable to map this code to a GID.
4. Use the encoding to map `c` to a name, use the Adobe Glyph List to map the
    name to unicode, and use a (3,1) "cmap" subtable to map this character to a
    GID.
5. Use the encoding to map `c` to a name, and use the "post" table to look
    up the glyph by name.

It is not completely specified which method should be used under which
circumstances.  The spec recommends to avoid use of simple TrueType fonts and
to use composite fonts with the `Encoding` set to `Identity-H` and
`CIDToGIDMap` set to `Identity`, instead.

This library uses the following methods when writing a TrueType font dictionary:

|             | non-symbolic | symbolic   |
| ----------: | :----------: | :--------: |
| encoding    | 4            | 2          |
| no encoding | avoid        | 1          |

The following methods are tried in order when reading a TrueType font dictionary:

|             | non-symbolic | symbolic   |
| ----------: | :----------: | :--------: |
| encoding    | 4, 3, 2, 1   | 4, 2, 5, 1 |
| no encoding | 2, 1         | 2, 1       |
