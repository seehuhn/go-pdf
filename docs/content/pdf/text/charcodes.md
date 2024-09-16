+++
title = 'Composite Fonts'
date = 2024-08-31T22:31:54+01:00
+++

# Composite Fonts

## Character Codes

*\[The following information is from section 9.7.6.3 (Handling undefined characters) of ISO 32000-2:2020.\]*

### Mapping Codes to CIDs

Valid codes are defined by the codespace ranges in the CMap.  If a code is
invalid, the following things happen:

1. CID 0 is returned.
2. The number of input bytes consumed is the length of the shortest code which
   contains the longest possible prefix of the given code.  In particular, if
   no valid code starts with the first byte of the given code, the number of
   bytes consumed is the length of the shortest code.

If the code is valid, the following things happen:
1. If there is a `cidchar` or `cidrange` mapping for the code,
   the resulting CID is returned.
2. Otherwise, if there is a `notdef` mapping for the code,
   the resulting CID is returned.
3. Otherwise, CID 0 is returned.

### Missing Glyphs

If the font does not contain a glyph for a CID, the following things happen:

1. If there is a notdef mapping for the corresponding code,
   and there is a glyph for the CID from the notdef mapping, this glyph is shown.
2. Otherwise, the glyph for CID 0 is shown.
