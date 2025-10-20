+++
title = 'TODO List'
date = 2024-08-31T18:43:34+01:00
weight = 100
+++

# TODO List

## Next Steps

- Implement property lists.
- Make Resource dictionaries file-independent.
- make dict.Type3 file-independent
- Try to merge `updateTextPosition` in "op-text.go" with the
  corresponding code from `processText` in "reader/reader.go".
- Get rid of `ResourceManagerEmbedFunc` and `EmbedHelperEmbedFunc`.
- Should graphics.NewWriter really take a ResourceManager argument?

## API

- Finalise the font API.
- Is the distinction between `pdf.Native` and `pdf.Object` really useful?
  Maybe we should just use `pdf.Object` everywhere?

## General

- fix infinite recursion risk in halftone Type 5 extraction
- test that we don't write numbers like `0.6000000000000001` in content streams
- By more systematic about the use of pdf.MalformedFileError, and in
  particular the `Loc` field there.
- avoid PDF output like `[-722 (1) -722] TJ` for centered text
- make a central list of all external assets/resources I include

## Fonts

- reconsider PostScriptName for *sfnt.Font.
- complete font subsetting implementation for all GSUB/GPOS subtable types
- implement missing cmap subtable formats (2, 8, 10, 13, 14)
- implement CFF font matrix handling
- re-introduce CFF subroutines optimization for better compression
- complete name table encoding implementations
- test that the widths of the `.notdef` character is correct for the
  standard 14 fonts
- double-check that I am correctly using the "Adobe Glyph List" and "Adobe
  Glyph List for New Fonts"
- improve font comparison for testing once better font equality is available

## Testing

- make sure that unit tests don't leave stray files behind
- re-enable and fix TextShowGlyphs test for space advance handling

## Missing Features

- implement high-quality text extraction
- implement JBIG2Decode filters
- implement JPXDecode filters
- implement Crypt filters
- allow for incremental updates to PDF files
- add a way to repair broken xref tables?
- implement public key encryption?
- should the library support FDF files?
