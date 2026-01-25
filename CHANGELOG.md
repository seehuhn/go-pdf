# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.7.0] - 2025-01-25

### Added
- **Complete PDF Action Support:** New `action` package implements all 19 PDF action types (GoTo, URI, JavaScript, SubmitForm, etc.) with typed Go structs and action triggers for annotations, pages, and forms.
- **Typed Destinations:** New `destination` package provides type-safe PDF destinations (XYZ, Fit, FitH, FitR, etc.) with validation.
- **Annotation Support:** New `annotation` package covers all PDF 2.0 annotation types with automatic fallback appearance generation for Text, FreeText, Link, Line, Square, and Circle annotations.
- **Content Streams as Objects:** New `graphics/content` package parses content streams into `[]Operator` slices that can be read, modified, and written between files. Includes builder subpackage for programmatic construction.
- **Optional Content (Layers):** New `oc` package for PDF 1.5+ layer support.
- **Typed Page Objects:** New `page` package with subpackages for box colors, navigation nodes, separation info, and transitions.
- **Text Extraction Improvements:** ActualText support for accessible text, new `pdf-extract` tool, and improved space detection.
- **PDF 2.0 Features:** RunLengthDecode filter, Lab8 image type, shading types 5-7, measure dictionaries, and collection support.

### Changed
- `color.DeviceGray`, `DeviceRGB`, `DeviceCMYK` are now types instead of functions.
- `font.Layouter.Encode()` simplified by removing width parameter.
- `annotation.Link` uses typed `Action` and `Destination` fields; `QuadPoints` changed from `[]float64` to `[]vec.Vec2`.
- `outline` package redesigned with typed destination and action fields.
- `GetResources` renamed to `ExtractResources`; `DecodeDict` deprecated.
- Merged `resource/` and `graphics/operator/` into `graphics/content/`.
- Unified graphics state management with typed state bits and blend modes.
- Type 3 font CharProcs now use `content.Stream`.
- Standardized extraction APIs to use `pdf.Extractor` parameter throughout.

### Fixed
- Stream decryption now applied lazily, enabling multiple reads from encrypted streams.
- Page tree reader more permissive with malformed Kids arrays.
- Various graphics state and color handling bugs.
- Numerous fuzzing failures addressed.

## [0.6.0] - 2025-06-30

### Added
- **Enhanced Graphics and Color Capabilities:** The library now has improved support for advanced graphics features, including more sophisticated PDF functions, shading patterns, and support for the sRGB color space, allowing for more accurate color reproduction.
- **API Modernization and New Features:** The library's public API has been updated and modernized for clarity and ease of use. This includes the introduction of a new `pdf.Getter` interface for accessing PDF data and the addition of new example programs like `pdf-concat` for merging PDF files.
- **Improved Text Handling and Extraction:** The way text is handled internally has been improved, including changes to how the library represents and encodes text. The text extraction example has also been updated to allow for more precise selection of text, such as by columns.

### Changed
- **Complete Font System Overhaul:** The library's font handling has been entirely rewritten with a new, more powerful, and flexible framework. This includes improved support for `Type1` fonts, better handling of character encodings (`CMap`), and more efficient subsetting of fonts to reduce file size.
- **Major Refactoring of Core PDF Structures:** The internal representation of PDF objects has been modernized, splitting them into `Native` and `Object` types for more robust handling. Additionally, the management of page content has been moved into a new `pagetree` package, simplifying the library's architecture.

## [0.5.0] - 2024-05-17

### Added
- New `pdf-inspect` tool for inspecting PDF files.
- New `metadata` example to demonstrate how to add XMP metadata to a PDF file.
- GitHub Actions for continuous integration.

### Changed
- Renamed `TextStart` to `TextBegin` to better match the PDF operator names.

### Fixed
- Many documentation fixes and improvements.
- Fixed several potential integer overflow and security issues.
- Updated dependencies.

## [v0.4.5] (2024-03-26)

### Changed
- Improved PDF string encoding algorithm with better parentheses balancing and more efficient buffering
- Refactored font rendering to use unified sfnt.Layouter API across all font types
- Updated dependencies to stable releases (postscript v0.4.5, sfnt v0.4.5)
- Simplified OpenType font file loading by using sfnt.ReadFile() instead of manual file handling

### Fixed
- Enhanced benchmarking accuracy by adding proper timer reset in layout tests

### Added
- Better test coverage for String PDF formatting behavior

## [v0.4.4] (2024-03-07)

### Added
- Automatic ligature support for Type1 builtin fonts (ff, fi, fl, ffi, ffl) when not using fixed-pitch fonts
- Memory reuse optimization throughout the text rendering pipeline to reduce allocations
- Support for parsing decimal numbers starting with '.' (e.g., `.1`, `+.1`, `-.1`)

### Changed
- **BREAKING**: Layout method API now takes a GlyphSeq buffer parameter and appends glyphs instead of creating new sequences
- Updated to Go 1.22 and latest dependency versions (postscript v0.4.4, sfnt v0.4.4)
- Improved memory allocation patterns in font layout operations

### Fixed
- Ligature advance width calculation in Type1 fonts now uses correct glyph metrics
- Decimal number parsing in PDF content streams for edge cases with leading decimal points
- Fuzz test error handling to prevent unnecessary test failures

## [v0.4.3] (2024-03-06)

### Changed
- **Breaking**: PDF version now required as explicit parameter in `NewWriter`, `Create`, and related functions instead of `WriterOptions.Version` field
- Enhanced text layout functionality with improved glyph positioning and simplified `TextLayout` method signature
- Updated PostScript and SFNT dependencies to v0.4.3
- Improved graphics state management with new methods for state copying, text layout, and position tracking

### Added
- `PadTo` method for `GlyphSeq` to add padding space
- New utility methods in graphics state handling: `TextLayout`, `ApplyTo`, `CopyTo`, and `GetTextPositionDevice`

### Removed
- `Version` field from `WriterOptions` struct (moved to explicit parameter)
- Deprecated content decoder module

### Fixed
- Cross-reference table parsing now correctly handles empty sections

## [v0.4.2] (2024-03-03)

### Changed
- **BREAKING**: Renamed `font.Embedder` interface to `font.Font` across all font packages
- **BREAKING**: Unified XObject API - replaced `DrawImage()` and `DrawFormXObject()` with single `DrawXObject()` method
- **BREAKING**: Major refactoring of pattern and shading APIs with new package organization (`graphics/pattern` and `graphics/shading`)
- **BREAKING**: Renamed font embedding types for consistency (`FontDict*` replaces `EmbedInfo*` across CFF, OpenType, TrueType, and Type1 packages)

### Added
- New `IsFixedPitch()` method in font geometry to detect monospace fonts
- Enhanced ligature support with improved text layout functionality
- New utility methods for `GlyphSeq`: `Text()` and `TotalWidth()`
- New `font.Dict` interface for low-level font dictionary operations
- Color space utility functions: `NumValues()`, `IsPattern()`, and `IsIndexed()`

### Fixed
- Improved OpenType font test handling by properly clearing GDEF, GSUB, and GPOS tables
- Enhanced text layout with better character spacing and ligature handling
