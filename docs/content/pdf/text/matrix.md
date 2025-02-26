+++
title = 'Font Matrices'
date = 2025-01-04T19:16:53Z
weight = 90
+++

# Font Matrices

Glyph outlines are designed in a font-dependent coordinate system,
sometimes called the *character space* or *design space*.
A *font matrix* is used to convert from the character
space to *text space* coordinates.

The font matrix is represented by a 6-element array
`[a b c d e f]`, corresponding to the transformation
which maps `x, y` in the character space to
`a*x + c*y + e, b*x + d*y + f` in text space.
A typical font matrix is given by `[0.001 0 0 0.001 0 0]`.

The go-pdf library uses the
[`seehuhn.de/go/geom/matrix.Matrix`](https://pkg.go.dev/seehuhn.de/go/geom@v0.0.0-20241008120652-aada228c2941/matrix#Matrix)
type to represent font matrices.
