This directory contains example programs which demonstrate how to
perform common tasks with go-pdf.

The code here is meant to be read and copied by users of the library.
It must be exceptionally clean and idiomatic, and the generated PDF
files should be efficient and of production quality.  Examples are
self-contained: they use only the public go-pdf API, without internal
helper packages.

Examples are grouped by API area at the top level (`annotation/`,
`font/`, `image/`, ...), with each example in a subdirectory named
after the task it demonstrates, such as `annotation/hover` or
`font/variable-ttf`.  Simple stand-alone examples like `hello-world`
may sit at the top level.

The scope of this directory is deliberately limited: each example is a
minimal demonstration of one task.  Larger showcase programs and
complete applications live outside the go-pdf repository.  Programs
which visually test library features belong in `../feature-tests`;
programs which probe viewer behavior belong in `../viewer-tests`.
