+++
title = 'Unit Tests'
date = 2024-12-19T11:58:24Z
draft = true
+++

Unit Tests
==========

PDF Files
---------

The package `seehuhn.de/go/pdf/internal/debug/memfile` provides support for
in-memory PDF files, which can be used in "round-trip" tests.
Use `memfile.NewPDFWriter` to create a new in-memory PDF file.

Fonts
-----

The package `seehuhn.de/go/pdf/internal/debug/makefont` can be used to create
test fonts in all formats supported by the library.
