+++
title = 'TODO List'
date = 2024-08-31T18:43:34+01:00
weight = 100
+++

# TODO List

## API

- Complete the transition from `cmap.Info` to `cmap.InfoNew`.
- Finalise the font API.
- decide whether matrices are `[6]float64` or `[]float64`

## General

- Rearrange go-pdf/examples to be more logical.
  In particular have subdirectories for the different types of examples:
  - testing viewer behaviour
  - testing library output for different PDF features
  - example code for users
- complete the support for writing human-readable PDF files
- Make sure that readers cannot get into infinite loops when resources
  depend on each other in a cycle.
- test that we don't write numbers like `0.6000000000000001` in content streams
- By more systematic about the use of pdf.MalformedFileError, and in
  particular the `Loc` field there.
- avoid PDF output like `[-722 (1) -722] TJ` for centered text

## Fonts

- test that the widths of the `.notdef` character is correct for the
  standard 14 fonts
- (optionally) write encodings that can be interpreted without reading
  the font program?

## Testing

- make sure that unit tests don't leave stray files behind
- when fuzzing PDF files, write the examples without compression
- look into the VeraPDF test suite, for inspiration/comparison

## Missing Features

- implement high-quality text extraction
- add support for non-embedded fonts
- implement the TIFF predictor functions for LZWDecode
- implement CCITTFaxDecode filters
- implement RunLengthDecode filters
- implement JBIG2Decode filters
- implement JPXDecode filters
- implement Crypt filters
- allow for incremental updates to PDF files?
- add a way to repair broken xref tables?
- implement public key encryption?
