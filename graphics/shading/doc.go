// Copyright 2024 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

// Package shading provides functionality for creating PDF shading objects.
// Shading objects define smooth color transitions and gradients in PDF documents.
// For a given shading, PDF viewers can compute the color of each output
// pixel at device resolution.
//
// This package supports multiple types of PDF shadings:
//   - Type1: Function-based shadings
//   - Type2: Axial shadings
//   - Type3: Radial shadings
//   - Type4: Free-form Gouraud-shaded triangle meshes
package shading
