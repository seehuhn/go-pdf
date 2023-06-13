Notes about PDF Fonts
=====================

# Supported Types of Fonts

## Simple PDF Fonts

Simple fonts always use a single byte per character.
Thus, only 256 distinct glyphs can be used, even if the font contains more
glyphs.

### Type1 Fonts

These use `Type1` as the `Subtype` in the font dictionary.
Font data is embedded via the `FontFile` entry in the font descriptor.
The 14 built-in standard fonts are of this type.

The `Encoding` entry in the font dictionary describes the mapping from
character codes to glyph names.

### Multiple Master Type1 Fonts

These are fonts which can be modified using one or more parameters (e.g.
weight, width, etc.). Multiple master Type1 fonts use `MMType1` as the
`Subtype` in the font dictionary.

### CFF Fonts (PDF 1.2)

These use `Type1` as the `Subtype` in the font dictionary.
Font data is embedded via the `FontFile3` entry in the font descriptor,
and the `Subtype` entry in the font file stream dictionary is `Type1C`.

Usually, `Encoding` is omitted from the font dictionary, and the mapping from
character codes to glyph names is described by the "builtin encoding" of the
CFF font.  The CFF data is not allowed to be CID-keyed, *i.e.* the CFF font
must not contain a `ROS` operator.

### CFF-based OpenType Fonts (PDF 1.6)

These use `Type1` as the `Subtype` in the font dictionary.
The font data is embedded via the `FontFile3` entry in the font descriptor,
and the `Subtype` entry in the font file stream dictionary is `OpenType`.

Usually, `Encoding` is omitted from the font dictionary, and the mapping from
character codes to glyph names is described by the "builtin encoding" of the
OpenType font.  The CFF data embedded in the OpenType font is not allowed to be
CID-keyed, *i.e.* the CFF font must not contain a `ROS` operator.

There seems little reasson to use this font type, since the OpenType wrapper
could be omitted and the CFF font data could directly be embedded as a CFF font.

### TrueType Fonts (PDF 1.1)

These use `TrueType` as the `Subtype` in the font dictionary.
The font data is embedded via the `FontFile2` entry in the font descriptor.
Only a subset of the TrueType tables is required for embedded fonts.

Usually, `Encoding` is omitted from the font dictionary, and a TrueType "cmap"
table describes the mapping from character codes to glyphs (see section 9.6.6.4
of PDF 32000-1:2008).

### Glyf-based OpenType Fonts (PDF 1.6)

These use `TrueType` as the `Subtype` in the font dictionary.
The font data is embedded via the `FontFile3` entry in the font descriptor,
and the `Subtype` entry in the font file stream dictionary is `OpenType`.

Usually, `Encoding` is omitted from the font dictionary, and a TrueType "cmap"
table describes the mapping from character codes to glyphs (see section 9.6.6.4
of PDF 32000-1:2008).

There seems little reasson to use this font type, since the font data
could equally be embedded as a TrueType font.

### Type3 Fonts

These use `Type3` as the `Subtype` in the font dictionary.
The font data is embedded via the `CharProcs` entry in the font dictionary.

The `Encoding` entry in the font dictionary describes the mapping from
character codes to glyph names (*i.e.* to the keys in the `CharProcs`
dictionary).



## PDF CIDFonts

CIDFonts can use multiple bytes to encode a character, the exact encoding is
configurable.  The most common encoding is `Identity-H` which uses two bytes
for every character.

### CFF Fonts (PDF 1.3)

These fonts use `Type0` as the `Subtype` in the font dictionary,
and `CIDFontType0` as the `Subtype` in the CIDFont dictionary.
The font data is embedded via the `FontFile3` entry in the font descriptor,
and the `Subtype` entry in the font file stream dictionary is `CIDFontType0C`.

The `Encoding` entry in the font dictionary specifies a CMap which
describes the mapping from character codes to CIDs.
If the CFF font is CID-keyed, *i.e.* if it contain a `ROS` operator,
then the `charset` table in the CFF font describes the mapping from CIDs to
glyphs.  Otherwise, the CID is used the glyph index directly.

### CFF-based OpenType Fonts (PDF 1.6)

These fonts use `Type0` as the `Subtype` in the font dictionary,
and `CIDFontType0` as the `Subtype` in the CIDFont dictionary.
The font data is embedded via the `FontFile3` entry in the font descriptor,
and the `Subtype` entry in the font file stream dictionary is `OpenType`.

The `Encoding` entry in the font dictionary specifies a CMap which
describes the mapping from character codes to CIDs.
If the CFF font is CID-keyed, *i.e.* if it contain a `ROS` operator,
then the `charset` table in the CFF font describes the mapping from CIDs to
glyphs.  Otherwise, the CID is used the glyph index directly.

There seems little reasson to use this font type, since the OpenType wrapper
could be omitted and the CFF font data could be embedded as a CFF font.

### TrueType Fonts (PDF 1.3)

These fonts use `Type0` as the `Subtype` in the font dictionary,
and `CIDFontType2` as the `Subtype` in the CIDFont dictionary.
The font data is embedded via the `FontFile2` entry in the font descriptor.
Only a subset of the TrueType tables is required for embedded fonts.

The `Encoding` entry in the font dictionary specifies a CMap which
describes the mapping from character codes to CIDs.  The `CIDToGIDMap`
entry in the CIDFont dictionary specifies the mapping from CIDs to glyphs.

### Glyf-based OpenType Fonts (PDF 1.6)

These fonts use `Type0` as the `Subtype` in the font dictionary,
and `CIDFontType2` as the `Subtype` in the CIDFont dictionary.
The font data is embedded via the `FontFile3` entry in the font descriptor,
and the `Subtype` entry in the font file stream dictionary is `OpenType`.

The `Encoding` entry in the font dictionary specifies a CMap which
describes the mapping from character codes to CIDs.  The `CIDToGIDMap`
entry in the CIDFont dictionary specifies the mapping from CIDs to glyphs.

There seems little reasson to use this font type, since the font data
could equally be embedded as a TrueType font.
