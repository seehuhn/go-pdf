+++
title = 'Notes about the API design'
date = 2024-08-31T23:09:46+01:00
weight = 10
+++

Notes about the API design
==========================

Here I will collect notes about the overall API design of the library.

Error Handling
--------------

- In case of malformed PDF files, the library should read as much information
  as possible and not return an error unless absolutely necessary.  Correct
  files shold be read as per the spec, incorrect files should be read on a
  best-effort basis.  This library is *not* a PDF checker.

- Where errors must be returned by readers, there should is a distinction
  between errors caused by malformed input files, and OS-level errors.
  Errors for malformed input wrap `pdf.MalformedFileError`.

- Functions for writing PDF data should refuse to write invalid PDF and
  should always abort with an error in case of invalid input.
  PDF written by this library should be 100% conformant to the
  PDF spec.

Naming
------

- I still need to decide on a consisten naming scheme for functions which
  extract data from a PDF file.  Currently I have the following:
  - Functions to extract basic types have names starting with `Get...`.
    Examples include:
    - [`GetName(r pdf.Getter, obj Object) (x pdf.Name, err error)`](https://pkg.go.dev/seehuhn.de/go/pdf#GetName)
    - [`GetString(r pdf.Getter, obj Object) (x pdf.String, err error)`](https://pkg.go.dev/seehuhn.de/go/pdf#GetString)
    - [`GetNumber(r pdf.Getter, obj Object) (x pdf.Number, err error)`](https://pkg.go.dev/seehuhn.de/go/pdf#GetNumber)
    - [`GetRectangle(r pdf.Getter, obj Object) (x *pdf.Rectangle, err error)`](https://pkg.go.dev/seehuhn.de/go/pdf#GetRectangle)
  - More complex structures often have names starting with extract.
    Examples include:
    - [`font.ExtractDescriptor(r pdf.Getter, obj pdf.Object) (*font.Descriptor, error)`](https://pkg.go.dev/seehuhn.de/go/pdf@v0.5.1-0.20240906163623-d591f7fad2df/font#ExtractDescriptor)
    - [`cff.ExtractSimple(r pdf.Getter, dicts *font.Dicts) (*cff.FontDictSimple, error)`](https://pkg.go.dev/seehuhn.de/go/pdf/font/cff#ExtractSimple)
    - [`color.ExtractSpace(r pdf.Getter, desc pdf.Object) (color.Space, error)`](https://pkg.go.dev/seehuhn.de/go/pdf@v0.5.1-0.20240906163623-d591f7fad2df/graphics/color#ExtractSpace)
  In all cases, the first argument is a `pdf.Getter`.
