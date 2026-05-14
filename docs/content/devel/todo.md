+++
title = 'TODO List'
date = 2024-08-31T18:43:34+01:00
weight = 100
+++

# TODO List

## Next Steps

- Make Resource dictionaries file-independent.
- make dict.Type3 file-independent
- Get rid of `ResourceManagerEmbedFunc` and `EmbedHelperEmbedFunc`.

## API

- Finalise the font API.
- Is the distinction between `pdf.Native` and `pdf.Object` really useful?
  Maybe we should just use `pdf.Object` everywhere?
- `pdf.Equal` falls back to pointer identity for `*Stream` and
  `*Placeholder` because content comparison would require I/O,
  decryption, and filter decoding. This leaks identity semantics into
  every caller that resolves references and then compares (e.g.
  `property.proxyList.Equal`, `generic.Object.Equal`). Introduce a
  `pdf.StreamsContentEqual(getter, a, b *Stream) (bool, error)` (or a
  pair `pdf.EqualShallow` / `pdf.EqualDeep`) and migrate the few
  callers that want true value equality, so the library has one
  consistent answer to "are these the same PDF value?".

## General

- consider removing the `github.com/xdg-go/stringprep` dependency.
  It is the only third-party (non-Go-team) external module in the tree
  and has a single call site in `crypto.go` (`utf8Passwd`, SASLprep for
  AESv5 / PDF 2.0 passwords).  The upstream project is dormant.
  PDF 2.0 (§7.6.4.3.3) calls for SASLprep (RFC 4013) by name, so
  replacements have to implement that algorithm; inlining a minimal
  SASLprep under `internal/` is the most direct option.
- test that we don't write numbers like `0.6000000000000001` in content streams
- By more systematic about the use of pdf.MalformedFileError, and in
  particular the `Loc` field there.
- avoid PDF output like `[-722 (1) -722] TJ` for centered text
- make a central list of all external assets/resources I include

## Fonts

- reconsider PostScriptName for *sfnt.Font.
- complete font subsetting implementation for all GSUB/GPOS subtable types
- implement missing cmap subtable formats (2, 8, 10, 13, 14)
- implement CFF font matrix handling
- re-introduce CFF subroutines optimization for better compression
- complete name table encoding implementations
- test that the widths of the `.notdef` character is correct for the
  standard 14 fonts
- double-check that I am correctly using the "Adobe Glyph List" and "Adobe
  Glyph List for New Fonts"
- improve font comparison for testing once better font equality is available

## Testing

- make sure that unit tests don't leave stray files behind
- re-enable and fix TextShowGlyphs test for space advance handling

## Missing Features

- implement JPXDecode filters
- implement Crypt filters
- allow for incremental updates to PDF files
- add a way to repair broken xref tables?
- implement public-key encryption (PDF spec §7.6.5).
  Currently the only supported `/Encrypt /Filter` is `Standard`; files
  using `Adobe.PubSec` (X.509-certificate-based encryption used in some
  enterprise workflows) are rejected by the reader.
- should the library support FDF files?
