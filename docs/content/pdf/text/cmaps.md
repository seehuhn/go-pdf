+++
title = 'Character Codes for Composite Fonts'
date = 2024-09-04T13:29:41+01:00
weight = 30
+++

# Character Codes for Composite Fonts

For composite fonts, the interpretation of character codes in a PDF content
stream is provided by a "CMap", which maps character codes to character
identifier (CID) value.  Depending on the "CID System" used, CID values
may have known semantics.  A separate table may map CID values to glyph
identifier (GID) values.

## Semantics

The `Encoding` entry in the font dictionary specifies a CMap which
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

If the font does not contain a glyph for a CID, the following things happen:

1. If the CMap contains a `notdef` mapping for the corresponding code, and
    there is a glyph for the CID from the `notdef` mapping, this glyph is shown.
2. Otherwise, the glyph for CID 0 is shown.


The `CIDSystemInfo` entry in the CIDFont dictionary and in the CMap specifies
a "character collection". A character collection maps CID values to
characters.  If a standard character collection is used, CID values
can be mapped to Unicode values.

## CMap data

A PDF CMap is a PostScript-based text file, which is embedded into the PDF
file as a stream.

There is a number of pre-defined CMaps which must be known to PDF readers and
thus can be omitted from the PDF files:
- The predefined CMaps include an "Identity" CMap, which maps 2-byte codes to
  16-bit CID values.  In this case, no interpretation of the CID values is
  implied.
- All other predefined CMaps imply an interpretation of the CID values.

## Mapping between CID and GID

If the font uses CFF-based glyph outlines, there are two cases:
- If the font use "CIDFont operators", the font contains the mapping between
  CID and GID values.  This mapping specifies the CID for every GID,
  and the font size is smallest, if consecutive CIDs map to consecutive GIDs.
- If the CFF font does not use CIDFont operators, the CID is used as the GID.
  The font size is smallest, if the unused `charset` field is set to
  the standard encoding.

If the font uses glyf-based glyph outlines, there are two cases:
- If the font is embedded, the `CIDToGIDMap` entry in the CIDFont dictionary is
  used to map CIDs to GIDs.  The table allows to specify the GID for every CID.
  PDF file size is minimal, if CID values equal GID values.  Otherwise, the
  storage size of the table is proportional to the number of CIDs used.
- If the font is not embedded, one of the pre-defined CMaps must be used. This
  implies an interpretation of CID values, which is used to map CID values to
  unicode values. The font's "cmap" table is used to map these unicode values
  to glyphs.

## References

- [Adobe CMap resources](https://github.com/adobe-type-tools/cmap-resources/)

- ISO 32000-2:2020,
  sections 9.7.5 (CMaps), 9.7.6.2 (CMap mapping), 9.7.6.3 (Handling undefined characters),
  and 9.10.3 (ToUnicode CMaps)

- "PostScript Language Reference", third edition.
  This describes CMap files in general, but has no information specific to ToUnicode CMaps.
  Particularly relevant is Section 5.11.4, "CMap Dictionaries".
  https://www.adobe.com/jp/print/postscript/pdfs/PLRM.pdf#page=396

- Adobe Technical Note #5014, "Adobe CMap and CID Font Files Specification" (June 1993).
  This describes CMap files in general, but has no information specific to ToUnicode CMaps.
  Section 1.6 discusses CMap names.\
  https://adobe-type-tools.github.io/font-tech-notes/pdfs/5014.CIDFont_Spec.pdf

- Adobe Technical Note #5099, "Developing CMap Resources for CID-Keyed Fonts" (March 2012).
  This describes CMap files in general, but has no information specific to ToUnicode CMaps.
  Section 1.6 discusses CMap names.\
  https://adobe-type-tools.github.io/font-tech-notes/pdfs/5099.CMapResources.pdf

- Adobe Technical Note #5411, "ToUnicode Mapping File Tutorial" (May 2003).
  Some of the information in this document seems to be incorrect.\
  https://pdfa.org/norm-refs/5411.ToUnicode.pdf

- https://github.com/pdf-association/pdf-issues/issues/344
