+++
title = 'TODO List'
date = 2024-08-31T18:43:34+01:00
weight = 100
+++

# TODO List

- Make sure that readers cannot get into infinite loops when resources
  depend on each other in a cycle.
- By more systematic about the use of pdf.MalformedFileError, and in
  particular the `Loc` field there.

- test that we don't write numbers like `0.6000000000000001` in content streams
- test that the widths of the notdef character is correct for the
  standard 14 fonts
- optionally write encodings that can be interpreted without reading
  the font program?
- avoid PDF output like `[-722 (1) -722] TJ` for centered text
- decide whether matrices are `[6]float64` or `[]float64`
- add support for external fonts
- add support for writing human-readable PDF files
- when fuzzing PDF files, write the examples without compression

- make sure that unit tests don't leave stray files behind
- make sure that supporting incremental updates to PDF files will not require
  major changes to the API

- implement CCITTFaxDecode filters
- implement public key encryption?
- add a way to repair broken xref tables?
