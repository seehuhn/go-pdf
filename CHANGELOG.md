# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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

## [0.4.5] - 2024-03-26

### Changed
- Improved string formatting to be more efficient and robust.
- Updated font handling and dependencies.
- Added fuzz testing for strings to improve security.

## [0.4.4] - 2024-03-07

### Changed
- The `Layout` API has been updated to be more memory-efficient by allowing the reuse of glyph buffers. This change improves performance by reducing memory allocations.
- The `TextShow` method no longer requires a buffer, simplifying the API and improving performance.

## [0.4.3] - 2024-03-06

### Changed
- The API for creating PDF files has been updated to take the PDF version as a direct argument, rather than as a field in the `WriterOptions` struct. This change makes the library more robust and future-proof.

### Fixed
- A bug that prevented empty sections in cross-reference tables from being parsed correctly has been fixed.
