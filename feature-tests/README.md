This directory contains code which generates PDF files to test the
library's support for different PDF features.

Each subdirectory is a small program which writes a PDF file
(conventionally `test.pdf`).  Viewing the file gives a quick visual
check that go-pdf's support for a feature works correctly: the correct
rendering is known in advance, and a wrong-looking page usually means a
bug in go-pdf.  Tests often exercise a range of options (line styles,
function types, ...); the aim is reasonable rather than complete
coverage.

The Go code must be idiomatic but can be terse; it is not meant to be
pedagogical.  Shared helper packages are fine.

Litmus test: it belongs here if what you learn from the file is whether
go-pdf built it correctly.  It belongs in `../viewer-tests` if what you
learn is what a viewer does — because the specification is ambiguous,
because viewers are known to disagree, or because the file is a survey
of viewer behaviour with no expected result at all.

Some viewers have bugs of their own.  Where a test is known to touch
one, say so on the generated page (`image/compression` does this).
