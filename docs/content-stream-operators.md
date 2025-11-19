# PDF Content Stream Operators

This document lists all operators that can appear in PDF content streams.

## General Graphics State

| Operator | Arguments | PDF Version | Description |
|----------|-----------|-------------|-------------|
| **q** | — | 1.0 | save graphics state |
| **Q** | — | 1.0 | restore graphics state |
| **cm** | a b c d e f | 1.0 | modify transformation matrix |
| **w** | lineWidth | 1.0 | set line width |
| **J** | lineCap | 1.0 | set line cap |
| **j** | lineJoin | 1.0 | set line join |
| **M** | miterLimit | 1.0 | set miter limit |
| **d** | dashArray dashPhase | 1.0 | set dash pattern |
| **ri** | intent | 1.1 | set rendering intent |
| **i** | flatness | 1.0 | set flatness tolerance |
| **gs** | dictName | 1.2 | set graphics state parameters |

## Special Graphics State

The **cm** operator (listed above under General Graphics State) modifies the current transformation matrix and is classified as a special graphics state operator in PDF versions prior to 2.0.

## Path Construction

| Operator | Arguments | PDF Version | Description |
|----------|-----------|-------------|-------------|
| **m** | x y | 1.0 | begin new subpath |
| **l** | x y | 1.0 | append line segment |
| **c** | x₁ y₁ x₂ y₂ x₃ y₃ | 1.0 | append cubic Bézier curve |
| **v** | x₂ y₂ x₃ y₃ | 1.0 | append Bézier curve (initial point replicated) |
| **y** | x₁ y₁ x₃ y₃ | 1.0 | append Bézier curve (final point replicated) |
| **h** | — | 1.0 | close subpath |
| **re** | x y width height | 1.0 | append rectangle |

## Path Painting

| Operator | Arguments | PDF Version | Description |
|----------|-----------|-------------|-------------|
| **S** | — | 1.0 | stroke path |
| **s** | — | 1.0 | close and stroke path |
| **f** | — | 1.0 | fill path (non-zero winding) |
| **F** | — | 1.0 | fill path (deprecated, equivalent to **f**) |
| **f*** | — | 1.0 | fill path (even-odd rule) |
| **B** | — | 1.0 | fill and stroke path (non-zero winding) |
| **B*** | — | 1.0 | fill and stroke path (even-odd rule) |
| **b** | — | 1.0 | close, fill and stroke (non-zero winding) |
| **b*** | — | 1.0 | close, fill and stroke (even-odd rule) |
| **n** | — | 1.0 | end path (no-op) |

## Clipping Paths

| Operator | Arguments | PDF Version | Description |
|----------|-----------|-------------|-------------|
| **W** | — | 1.0 | set clipping path (non-zero winding) |
| **W*** | — | 1.0 | set clipping path (even-odd rule) |

## Text Objects

| Operator | Arguments | PDF Version | Description |
|----------|-----------|-------------|-------------|
| **BT** | — | 1.0 | begin text object |
| **ET** | — | 1.0 | end text object |

## Text State

| Operator | Arguments | PDF Version | Description |
|----------|-----------|-------------|-------------|
| **Tc** | charSpace | 1.0 | set character spacing |
| **Tw** | wordSpace | 1.0 | set word spacing |
| **Tz** | scale | 1.0 | set horizontal scaling |
| **TL** | leading | 1.0 | set text leading |
| **Tf** | font size | 1.0 | set font and size |
| **Tr** | render | 1.0 | set text rendering mode |
| **Ts** | rise | 1.0 | set text rise |

## Text Positioning

| Operator | Arguments | PDF Version | Description |
|----------|-----------|-------------|-------------|
| **Td** | tx ty | 1.0 | move text position |
| **TD** | tx ty | 1.0 | move text position and set leading |
| **Tm** | a b c d e f | 1.0 | set text matrix |
| **T*** | — | 1.0 | move to next line |

## Text Showing

| Operator | Arguments | PDF Version | Description |
|----------|-----------|-------------|-------------|
| **Tj** | string | 1.0 | show text string |
| **TJ** | array | 1.0 | show text with positioning |
| **'** | string | 1.0 | move to next line and show text |
| **"** | aw ac string | 1.0 | set spacing and show text |

## Type 3 Fonts

| Operator | Arguments | PDF Version | Description |
|----------|-----------|-------------|-------------|
| **d0** | wx wy | 1.0 | set glyph width (coloured glyph) |
| **d1** | wx wy llx lly urx ury | 1.0 | set glyph width and bounding box |

## Colour

| Operator | Arguments | PDF Version | Description |
|----------|-----------|-------------|-------------|
| **CS** | name | 1.1 | set stroke colour space |
| **cs** | name | 1.1 | set fill colour space |
| **SC** | c₁ ... cₙ | 1.1 | set stroke colour |
| **SCN** | c₁ ... cₙ [name] | 1.2 | set stroke colour (ICCBased, Separation, DeviceN, Pattern) |
| **sc** | c₁ ... cₙ | 1.1 | set fill colour |
| **scn** | c₁ ... cₙ [name] | 1.2 | set fill colour (ICCBased, Separation, DeviceN, Pattern) |
| **G** | gray | 1.0 | set stroke gray level |
| **g** | gray | 1.0 | set fill gray level |
| **RG** | r g b | 1.0 | set stroke RGB colour |
| **rg** | r g b | 1.0 | set fill RGB colour |
| **K** | c m y k | 1.0 | set stroke CMYK colour |
| **k** | c m y k | 1.0 | set fill CMYK colour |

## Shading Patterns

| Operator | Arguments | PDF Version | Description |
|----------|-----------|-------------|-------------|
| **sh** | name | 1.3 | paint shading |

## Inline Images

| Operator | Arguments | PDF Version | Description |
|----------|-----------|-------------|-------------|
| **BI** | — | 1.0 | begin inline image |
| **ID** | — | 1.0 | begin image data |
| **EI** | — | 1.0 | end inline image |

## XObjects

| Operator | Arguments | PDF Version | Description |
|----------|-----------|-------------|-------------|
| **Do** | name | 1.0 | paint XObject |

## Marked Content

| Operator | Arguments | PDF Version | Description |
|----------|-----------|-------------|-------------|
| **MP** | tag | 1.2 | marked content point |
| **DP** | tag properties | 1.2 | marked content point with properties |
| **BMC** | tag | 1.2 | begin marked content sequence |
| **BDC** | tag properties | 1.2 | begin marked content sequence with properties |
| **EMC** | — | 1.2 | end marked content sequence |

## Compatibility

| Operator | Arguments | PDF Version | Description |
|----------|-----------|-------------|-------------|
| **BX** | — | 1.1 | begin compatibility section |
| **EX** | — | 1.1 | end compatibility section |

## Notes

### Argument Types

Arguments can be:
- **number**: integer or real number
- **name**: PDF name object (e.g., `/DeviceRGB`)
- **string**: PDF string object (literal or hexadecimal)
- **array**: PDF array object
- **dict**: PDF dictionary object (inline only for certain operators)

### Operand Details

- **cm**: The six numbers a, b, c, d, e, f represent a transformation matrix [a b c d e f].
- **d**: dashArray is an array of numbers; dashPhase is a number.
- **SC**, **sc**, **SCN**, **scn**: The number of color components (c₁ ... cₙ) depends on the current colour space.
- **SCN**, **scn**: For Pattern colour spaces, an optional name argument specifies the pattern.
- **TJ**: The array contains strings and numbers (for text positioning adjustments).
- **"**: aw is word spacing, ac is character spacing.
- **Tm**: The six numbers a, b, c, d, e, f represent a text matrix [a b c d e f].

### Version Information

Operators without a specified PDF version were introduced in PDF 1.0. Operators with version numbers require at least that PDF version.

### Deprecated Operators

- **F**: Use **f** instead (deprecated in PDF 2.0).
- **Transfer functions**, **halftones**, and related operators in graphics state parameter dictionaries are deprecated in PDF 2.0.

## References

This document is based on ISO 32000-2:2020 (PDF 2.0) and the implementation in the seehuhn.de/go/pdf library.

Key specification sections:
- Table 50 — Operator categories (§8.2)
- Table 56 — Graphics state operators (§8.4.4)
- Table 58 — Path construction operators (§8.5.2)
- Table 59 — Path-painting operators (§8.5.3)
- Table 60 — Clipping path operators (§8.5.4)
- Table 73 — Colour operators (§8.6.8)
- Table 76 — Shading operator (§8.7.4.2)
- Table 86 — XObject operator (§8.8)
- Table 90 — Inline image operators (§8.9.7)
- Table 103 — Text state operators (§9.3)
- Table 105 — Text object operators (§9.4)
- Table 106 — Text-positioning operators (§9.4.2)
- Table 107 — Text-showing operators (§9.4.3)
- Table 111 — Type 3 font operators (§9.6.4)
- Table 352 — Marked-content operators (§14.6)
- Table 33 — Compatibility operators (§7.8.2)
