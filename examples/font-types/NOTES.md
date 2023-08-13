Supported Fonts Types in PDF Files
==================================

# Simple PDF Fonts

Simple fonts always use a single byte per character in the PDF content stream.
Thus, only 256 distinct glyphs can be used, even if the font contains more
glyphs.

## Type 1 Fonts

These fonts use `Type1` as the `Subtype` in the font dictionary.
Font data is embedded via the `FontFile` entry in the font descriptor.

If the encoding used in PDF content streams is different from the font's
builtin encoding, the `Encoding` entry in the font dictionary describes the
mapping from character codes to glyph names,

## Builtin Fonts

There are 14 fonts which are built into every PDF viewer.  These fonts
have standardised names.  They use `Type1` as the `Subtype` in the font dictionary,
but no font data needs to be embedded and (for PDF versions before 2.0)
neither glyph width information nor a font descriptor are required.

If the encoding used in PDF content streams is different from the font's
builtin encoding, the `Encoding` entry in the font dictionary describes the
mapping from character codes to glyph names,

## Simple CFF Fonts (PDF 1.2)

CFF fonts use `Type1` as the `Subtype` in the font dictionary.
Font data is embedded via the `FontFile3` entry in the font descriptor,
and the `Subtype` entry in the font file stream dictionary is `Type1C`.

The CFF data is not allowed to be CID-keyed, *i.e.* the CFF font must not
contain a `ROS` operator.  Usually, `Encoding` is omitted from the font
dictionary, and the mapping from character codes to glyphs is described by the
"builtin encoding" of the CFF font.

## Simple CFF-based OpenType Fonts (PDF 1.6)

These fonts use `Type1` as the `Subtype` in the font dictionary.
The font data is embedded via the `FontFile3` entry in the font descriptor,
and the `Subtype` entry in the font file stream dictionary is `OpenType`.

The CFF data embedded in the OpenType font is not allowed to be CID-keyed,
*i.e.* the CFF font must not contain a `ROS` operator.  Usually, `Encoding` is
omitted from the font dictionary, and the mapping from character codes to
glyphs is described by the "builtin encoding" of the OpenType font.

There seems little reason to use this font type, since the CFF font data
can be embedded directly without the OpenType wrapper.

## Multiple Master Fonts

These are Type1 fonts which can be modified using one or more parameters
(weight, width, *etc.*).  Multiple Master fonts use `MMType1` as the
`Subtype` in the font dictionary.

Multiple Master Fonts are not supported by this library.

## Simple TrueType Fonts (PDF 1.1)

These fonts use `TrueType` as the `Subtype` in the font dictionary.
The font data is embedded via the `FontFile2` entry in the font descriptor.
Only a subset of the TrueType tables is required for embedded fonts.

Usually, `Encoding` is omitted from the font dictionary, and the mapping from
character codes to glyphs is described by a `cmap` table in the TrueType font.

## Glyf-based OpenType Fonts (PDF 1.6)

These fonts use `TrueType` as the `Subtype` in the font dictionary.
The font data is embedded via the `FontFile3` entry in the font descriptor,
and the `Subtype` entry in the font file stream dictionary is `OpenType`.

The encoding used in PDF content streams is described by a combination of
the `Encoding` entry in the font dictionary, the `cmap` table in the TrueType,
and the symbolic/nonsymbolic flags in the font descriptor.

There seems little reason to use this font type, since the font data
could equally be embedded as a TrueType font.  Glyf-based OpenType fonts
seem not to be supported by the MacOS Preview application.

## Type 3 Fonts

These fonts use `Type3` as the `Subtype` in the font dictionary.
The font data is embedded via the `CharProcs` entry in the font dictionary.

The `Encoding` entry in the font dictionary describes the mapping from
character codes to glyph names (*i.e.* to the keys in the `CharProcs`
dictionary).



# Composite PDF Fonts

CIDFonts can use multiple bytes to encode a character, the exact encoding is
configurable.  The most common encoding is `Identity-H` which uses two bytes
for every character.

## Composite CFF Fonts (PDF 1.3)

These fonts use `Type0` as the `Subtype` in the font dictionary,
and `CIDFontType0` as the `Subtype` in the CIDFont dictionary.
The font data is embedded via the `FontFile3` entry in the font descriptor,
and the `Subtype` entry in the font file stream dictionary is `CIDFontType0C`.

The `Encoding` entry in the font dictionary specifies a PDF CMap which
describes the mapping from character codes to CIDs.
If the CFF font is CID-keyed, *i.e.* if it contain a `ROS` operator,
then the `charset` table in the CFF font describes the mapping from CIDs to
glyphs.  Otherwise, the CID is used as the glyph index directly.

## Composite CFF-based OpenType Fonts (PDF 1.6)

These fonts use `Type0` as the `Subtype` in the font dictionary,
and `CIDFontType0` as the `Subtype` in the CIDFont dictionary.
The font data is embedded via the `FontFile3` entry in the font descriptor,
and the `Subtype` entry in the font file stream dictionary is `OpenType`.
Only a subset of the OpenType tables is required for embedded fonts.

The `Encoding` entry in the font dictionary specifies a PDF CMap which
describes the mapping from character codes to CIDs.
If the CFF font is CID-keyed, *i.e.* if it contain a `ROS` operator,
then the `charset` table in the CFF font describes the mapping from CIDs to
glyphs.  Otherwise, the CID is used as the glyph index directly.

There seems little reason to use this font type, since the OpenType wrapper
could be omitted and the CFF font data could be embedded as a CFF font.

## Composite TrueType Fonts (PDF 1.3)

These fonts use `Type0` as the `Subtype` in the font dictionary,
and `CIDFontType2` as the `Subtype` in the CIDFont dictionary.
The font data is embedded via the `FontFile2` entry in the font descriptor.
Only a subset of the TrueType tables is required for embedded fonts.

The `Encoding` entry in the PDF font dictionary specifies a PDF CMap which
describes the mapping from character codes to CIDs.  The `CIDToGIDMap`
entry in the CIDFont dictionary specifies the mapping from CIDs to glyphs.

## Composite Glyf-based OpenType Fonts (PDF 1.6)

These fonts use `Type0` as the `Subtype` in the font dictionary,
and `CIDFontType2` as the `Subtype` in the CIDFont dictionary.
The font data is embedded via the `FontFile3` entry in the font descriptor,
and the `Subtype` entry in the font file stream dictionary is `OpenType`.
Only a subset of the OpenType tables is required for embedded fonts.

The `Encoding` entry in the font dictionary specifies a PDF CMap which
describes the mapping from character codes to CIDs.  The `CIDToGIDMap`
entry in the CIDFont dictionary specifies the mapping from CIDs to glyphs.

There seems little reason to use this font type, since the font data
could equally be embedded as a TrueType font.
