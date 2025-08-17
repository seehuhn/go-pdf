+++
title = 'TODO List'
date = 2024-08-31T18:43:34+01:00
weight = 100
+++

# TODO List

## API

- Finalise the font API.
- Is the distinction between `pdf.Native` and `pdf.Object` really useful?
  Maybe we should just use `pdf.Object` everywhere?

## General

- Make sure that readers cannot get into infinite loops when resources
  depend on each other in a cycle.
- test that we don't write numbers like `0.6000000000000001` in content streams
- By more systematic about the use of pdf.MalformedFileError, and in
  particular the `Loc` field there.
- avoid PDF output like `[-722 (1) -722] TJ` for centered text
- make a central list of all external assets/resources I include
- should the library support FDF files?

## Fonts

- reconsider PostScriptName for *sfnt.Font.
- test that the widths of the `.notdef` character is correct for the
  standard 14 fonts
- double-check that I am correctly using the "Adobe Glyph List" and "Adobe
  Glyph List for New Fonts"

## Testing

- make sure that unit tests don't leave stray files behind
- when fuzzing PDF files, write the examples without compression
- look into the VeraPDF test suite, for inspiration/comparison

## Missing Features

- implement high-quality text extraction
- implement RunLengthDecode filters
- implement JBIG2Decode filters
- implement JPXDecode filters
- implement Crypt filters
- allow for incremental updates to PDF files
- add a way to repair broken xref tables?
- implement public key encryption?
