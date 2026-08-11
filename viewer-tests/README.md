This directory contains code which generates PDF files to explore the
behavior of different PDF viewers.

The file is a probe: the point is to open it (conventionally `test.pdf`)
in several viewers and record what each one does.  There are several
reasons to want to know:

- the specification is silent or ambiguous, so no answer is authoritative;
- the specification is clear but viewers are known to disagree;
- neither: the file simply surveys which strategy each viewer uses, so that
  go-pdf can write files that work everywhere.

The last kind has no expected result at all.

There are no constraints on the Go source code or on the structure of
the generated PDF, but the rendered pages must be cleanly laid out and
must explain, on the page itself, what is being tested and how to
interpret the result.  Shared helper packages are fine.

Litmus test: it belongs here if what you learn from the file is what a
viewer does.  It belongs in `../feature-tests` if what you learn is
whether go-pdf built the file correctly.
