// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

// Package image provides support for embedding images in PDF documents.
//
// The package supports various image formats and provides facilities for
// image masking, transparency, and color space handling. Images can be
// created from Go's standard image.Image interface or constructed manually
// using the provided types.
//
// Key types:
//   - [Dict]: represents a standard PDF image with full color support
//   - [Mask]: represents a 1-bit image mask for transparency effects
//   - [SoftMask]: represents a grayscale mask for graduated transparency
//   - [Indexed]: represents an image with an indexed color palette
//
// The package handles compression automatically and supports both lossless
// and lossy storage formats.
package image
