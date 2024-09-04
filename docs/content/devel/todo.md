+++
title = 'TODO List'
date = 2024-08-31T18:43:34+01:00
weight = 100
+++

# TODO List

## General

- add support for writing human-readable PDF files
- Make sure that readers cannot get into infinite loops when resources
  depend on each other in a cycle.
- test that we don't write numbers like `0.6000000000000001` in content streams
- By more systematic about the use of pdf.MalformedFileError, and in
  particular the `Loc` field there.
- avoid PDF output like `[-722 (1) -722] TJ` for centered text
- when fuzzing PDF files, write the examples without compression
- decide whether matrices are `[6]float64` or `[]float64`

## Fonts

- add support for external fonts
- test that the widths of the `.notdef` character is correct for the
  standard 14 fonts
- optionally write encodings that can be interpreted without reading
  the font program?

## Stream Filters

- implement the TIFF predictor functions
- implement CCITTFaxDecode filters

## Features

- implement public key encryption?
- add a way to repair broken xref tables?
- allow for incremental updates to PDF files?

## Repository

- make sure that unit tests don't leave stray files behind
