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

- Functions for writing PDF data should refuse to write invalid PDF and
  should always abort with an error in case of invalid input.
  PDF written by this library should be 100% conformant to the
  PDF spec.

- Where errors must be returned by readers, there should is a distinction
  between errors caused by malformed input files, and OS-level errors.
  Errors for malformed input wrap `pdf.MalformedFileError`.

Naming
------

- Functions which read structured information from a pdf file
  have names starting with `Extract`.  The first argument is a `pdf.Getter`.
