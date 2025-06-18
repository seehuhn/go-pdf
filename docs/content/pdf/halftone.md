+++
title = 'Halftone Screening'
date = 2025-06-18T13:53:00+01:00
+++

# Halftone

## Overview

[Halftone Screening](https://en.wikipedia.org/wiki/Halftone) approximates
continuous-tone colors on devices that can only produce discrete colors. This
technique simulates unavailable colors by using patterns of pixels in the
device's available colors.

The process divides device space into a grid of halftone cells. On bilevel
devices where each pixel is either black or white, each cell simulates a gray
shade by selectively painting pixels black or white. The gray level within a
cell equals the ratio of white pixels to total pixels. A cell with n pixels can
render n + 1 different gray levels, from all black to all white. For a
requested gray value g (0.0 to 1.0), the number of white pixels is calculated
as floor(g × n).

Halftone screening is available in PDF 1.2 and later.

## PDF Halftone Representation in Graphics State

PDF graphics state contains two halftone-related entries:

**Halftone (`HT`)** specifies the current halftone and accepts these values:
- A dictionary for type 1 or 5 halftones
- A stream for type 6, 10, or 16 halftones
- The name `/Default` for device-specific default halftone (initial value)

**Halftone origin (`HTO`)** specifies the origin point for the halftone cell grid as an array of two numbers [x, y]. The initial value is device dependent.

## Halftone Types

All halftone types support transfer functions that override default gamma correction for specific colorants. Transfer functions can be the name `/Identity` for no correction, or Type 0 (sampled) or Type 4 (calculated) function objects.

### Type 1: Spot Function Based

Type 1 halftones define screens using frequency, angle, and spot function parameters.

| Key | Type | Description |
|-----|------|-------------|
| `Type` | name | (Optional) Must be `Halftone` if present. |
| `HalftoneType` | integer | Must be `1`. |
| `HalftoneName` | byte string | (Optional) The name of the halftone dictionary. |
| `Frequency` | number | Screen frequency in halftone cells per inch. |
| `Angle` | number | Screen angle in degrees counterclockwise relative to the device coordinate system. |
| `SpotFunction` | function, name, or array | Function defining pixel adjustment order for gray levels, or predefined spot function name. Since PDF 2.0, can be an array of names (processor uses first recognized). |
| `AccurateScreens` | boolean | Enables precise but computationally expensive halftone algorithm. Default: `false`. |
| `TransferFunction` | function or name | (Optional) Overrides current transfer function for this component. Use `Identity` for identity function. |

To implement Type 1 halftones, convert the designer's frequency (cells/inch), angle (degrees), and spot function into dictionary entries. Custom functions f(x,y) must be converted to Type 4 PostScript calculator functions. During rendering, create halftone cells based on frequency and angle, assign pixel coordinates in the [-1.0, 1.0] range, and evaluate the spot function for each pixel to determine whitening order. For any gray level, sort pixels by spot function value and whiten the appropriate number.

Predefined spot functions include: SimpleDot, InvertedSimpleDot, DoubleDot, InvertedDoubleDot, CosineDot, Double, InvertedDouble, Line, LineX, LineY, Round, Ellipse, EllipseA, InvertedEllipseA, EllipseB, EllipseC, InvertedEllipseC, Square, Cross, Rhomboid, and Diamond.

### Type 5: Multi-Colorant

Type 5 halftones define separate screens for multiple colorants.

| Key | Type | Description |
|-----|------|-------------|
| `Type` | name | (Optional) Must be `Halftone` if present. |
| `HalftoneType` | integer | Must be `5`. |
| `HalftoneName` | byte string | (Optional) The name of the halftone dictionary. |
| `Default` | dictionary or stream | Halftone for colorants without specific entries. Must not be Type 5. Required to have transfer function if nonprimary colorants exist. |
| *colorant name* | dictionary or stream | (One per colorant) Halftone for the named colorant. May be any type except 5. Standard names: `Cyan`, `Magenta`, `Yellow`, `Black` (CMYK); `Red`, `Green`, `Blue` (RGB); `Gray` (DeviceGray). Spot colors use specific colorant names. |

Create a dictionary with colorant names as keys and halftone dictionaries
(Types 1, 6, 10, or 16) as values. Include a Default entry for unspecified
colorants, with each component halftone translated according to its type. Child
halftone dicts for nonprimary or nonstandard primary components must specify a
transfer function.

During rendering, select the appropriate halftone based on the current colorant
using standard names for process colors and colorant names from Separation or
DeviceN color spaces for spot colors. Apply the default halftone for any
colorant without a specific entry.

### Type 6: Threshold Array

Type 6 halftones use PDF streams with zero screen angle.

| Key | Type | Description |
|-----|------|-------------|
| `Type` | name | (Optional) Must be `Halftone` if present. |
| `HalftoneType` | integer | Must be `6`. |
| `HalftoneName` | byte string | (Optional) The name of the halftone dictionary. |
| `Width` | integer | Threshold array width in device pixels. |
| `Height` | integer | Threshold array height in device pixels. |
| `TransferFunction` | function or name | (Optional) Overrides current transfer function. Use `Identity` for identity function. |

The stream contains Width × Height bytes, each with an 8-bit threshold value (0-255). Values are stored in row-major order with horizontal coordinates changing faster than vertical. The first value corresponds to device coordinates (0, 0). Convert the designer's 2D threshold array into a stream by serializing values in row-major order. During rendering, tile the threshold array across device space. For pixel (x, y), find the threshold at (x mod Width, y mod Height). Paint the pixel black if the gray level is less than the threshold, white if greater or equal. Treat threshold value 0 as 1.

### Type 10: Angled Threshold Array

Type 10 halftones use PDF streams supporting non-zero screen angles through two-square decomposition.

| Key | Type | Description |
|-----|------|-------------|
| `Type` | name | (Optional) Must be `Halftone` if present. |
| `HalftoneType` | integer | Must be `10`. |
| `HalftoneName` | byte string | (Optional) The name of the halftone dictionary. |
| `Xsquare` | integer | Side of square X in device pixels. Horizontal displacement between corresponding points in adjacent halftone cells. |
| `Ysquare` | integer | Side of square Y in device pixels. Vertical displacement between corresponding points in adjacent halftone cells. |
| `TransferFunction` | function or name | (Optional) Overrides current transfer function. Use `Identity` for identity function. |

The stream format contains Xsquare² + Ysquare² bytes. The first Xsquare² bytes represent the first square, followed by Ysquare² bytes for the second square, both in row-major order. Convert frequency and angle to square dimensions using: frequency = resolution / √(Xsquare² + Ysquare²) and angle = atan(Ysquare / Xsquare). The two squares tile together to cover device space, with the last row of the first square adjacent to the first row of the second square in the same column.

### Type 16: High-Precision Threshold Array

Type 16 halftones use PDF streams with 16-bit threshold values for higher precision.

| Key | Type | Description |
|-----|------|-------------|
| `Type` | name | (Optional) Must be `Halftone` if present. |
| `HalftoneType` | integer | Must be `16`. |
| `HalftoneName` | byte string | (Optional) The name of the halftone dictionary. |
| `Width` | integer | Width of the first (or only) rectangle in device pixels. |
| `Height` | integer | Height of the first (or only) rectangle in device pixels. |
| `Width2` | integer | (Optional) Width of the second rectangle. Requires `Height2` if present. |
| `Height2` | integer | (Optional) Height of the second rectangle. Requires `Width2` if present. |
| `TransferFunction` | function or name | (Optional) Overrides current transfer function. Use `Identity` for identity function. |

Each threshold value uses 2 bytes in big-endian order. For one rectangle: 2 × Width × Height bytes. For two rectangles: 2 × (Width × Height + Width2 × Height2) bytes. This format is similar to Type 6 but with 16-bit values providing 65,536 gray levels instead of 256. Rendering follows Type 10 principles but with 16-bit threshold comparisons. When two rectangles are specified, they tile to ensure complete coverage without overlaps.

Type 16 halftone screening is available in PDF 1.3 and later.

## Rendering

All halftone screening occurs in device space, unaffected by the current
transformation matrix (CTM). Before halftone screening, convert all color
components to additive form where larger values represent lighter colors: 0.0
is black and 1.0 is white. For subtractive color spaces like CMYK, invert the
values so 0% ink coverage becomes 1.0 (white) and 100% ink coverage becomes 0.0
(black).
