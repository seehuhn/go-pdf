+++
title = 'PDF Functions'
date = 2025-06-18T13:53:00+01:00
+++

# PDF Functions

## Overview

PDF functions are parameterized mathematical transformations that map \(m\) input
values to \(n\) output values. These static, self-contained numerical
transformations are used throughout PDF for color transformations, halftone
spot functions, smooth shadings, and other mathematical operations.

Functions operate within defined domains (valid input ranges) and optionally ranges (valid output ranges). Input values are automatically clipped to the domain, and output values are clipped to the range. All inputs and outputs must be numbers, and functions have no side effects.

PDF functions are available in PDF 1.2 and later, with Types 2 and 3 introduced in PDF 1.3.

## PDF Function Representation

PDF functions are represented as either dictionaries or streams, depending on the function type:

### Type 0: Sampled Functions

Type 0 functions use a table of sample values with interpolation to approximate
functions with bounded domains and ranges. They are represented as a stream.
The stream dict contains the following keys:

| Key | Type | Description |
|-----|------|-------------|
| `FunctionType` | integer | Must be `0`. |
| `Domain` | array | Array of \(2m\) numbers defining input ranges. For each input \(i\), Domain[2i] ≤ \(x_i\) ≤ Domain[2i+1]. |
| `Range` | array | Array of \(2n\) numbers defining output ranges. For each output \(j\), Range[2j] ≤ \(y_j\) ≤ Range[2j+1]. |
| `Size` | array | Array of \(m\) positive integers specifying number of samples in each input dimension. |
| `BitsPerSample` | integer | Bits per sample value. Valid values: 1, 2, 4, 8, 12, 16, 24, 32. |
| `Order` | integer | (Optional) Interpolation order. 1 for linear, 3 for cubic spline. Default: 1. |
| `Encode` | array | (Optional) Array of \(2m\) numbers for linear mapping of inputs to sample table indices. Default: [0 (\(\text{Size}_0-1\)) 0 (\(\text{Size}_1-1\))...]. |
| `Decode` | array | (Optional) Array of \(2n\) numbers for linear mapping of samples to output range. Default: same as Range. |

The stream contains \(\text{Size}_0 \times \text{Size}_1 \times \ldots \times \text{Size}_{m-1}\) sample values, each using BitsPerSample bits. Samples are packed continuously with no padding at byte boundaries. For multidimensional input, the first dimension varies fastest. For multidimensional output, values are stored in Range order.

Type 0 functions clip inputs to domain, encode to sample indices, interpolate between nearest samples, decode the result, and clip to range.

### Type 2: Power Interpolation Functions

Type 2 functions define power interpolation: \(y = C0 + x^N × (C1 - C0)\).
The PDF specification refers to this as "exponential interpolation".

| Key | Type | Description |
|-----|------|-------------|
| `FunctionType` | integer | Must be `2`. |
| `Domain` | array | Array of 2 numbers defining input range. \(\text{Domain}[0] \leq x \leq \text{Domain}[1]\). |
| `Range` | array | (Optional) Array of \(2 \times n\) numbers defining output ranges. For each output \(j\), \(\text{Range}[2j] \leq y_j \leq \text{Range}[2j+1]\). |
| `C0` | array | (Optional) Array of \(n\) numbers defining function result when \(x = 0.0\). Default: [0.0]. |
| `C1` | array | (Optional) Array of \(n\) numbers defining function result when \(x = 1.0\). Default: [1.0]. |
| `N` | number | The interpolation exponent. |

Domain must ensure \(x \geq 0\) if \(N\) is non-integer, and \(x \neq 0\) if \(N\) is negative. When \(N = 1\), the function performs linear interpolation between \(C0\) and \(C1\). Each output component \(j\) is calculated as: \(y_j = C0_j + x^N \times (C1_j - C0_j)\).

### Type 3: Piecewise Defined Functions

Type 3 functions combine multiple 1-input functions across different subdomains to create a single function. The PDF specification refers to these as "stitching functions".

| Key | Type | Description |
|-----|------|-------------|
| `FunctionType` | integer | Must be `3`. |
| `Domain` | array | Array of 2 numbers defining input range. \(\text{Domain}[0] \leq x \leq \text{Domain}[1]\). |
| `Range` | array | (Optional) Array of \(2 \times n\) numbers defining output ranges. For each output \(j\), \(\text{Range}[2j] \leq y_j \leq \text{Range}[2j+1]\). |
| `Functions` | array | Array of \(k\) 1-input functions to be combined. All must have same output dimensionality. |
| `Bounds` | array | Array of \(k-1\) numbers defining subdomain boundaries. Must be in increasing order within Domain. |
| `Encode` | array | Array of \(2 \times k\) numbers mapping each subdomain to corresponding function's domain. |

The domain is partitioned into \(k\) subdomains using Bounds values. The first subdomain is \([\text{Domain}_0, \text{Bounds}_0)\), intermediate subdomains are \([\text{Bounds}_i, \text{Bounds}_{i+1})\), and the last is \([\text{Bounds}_{k-2}, \text{Domain}_1]\). Special cases apply when \(\text{Domain}_0 = \text{Bounds}_0\). Each input value is mapped to its subdomain, encoded using the Encode array, then passed to the corresponding function.

### Type 4: PostScript Calculator Functions

Type 4 functions use a subset of PostScript language to define arbitrary calculations.

| Key | Type | Description |
|-----|------|-------------|
| `FunctionType` | integer | Must be `4`. |
| `Domain` | array | Array of \(2 \times m\) numbers defining input ranges. For each input \(i\), \(\text{Domain}[2i] \leq x_i \leq \text{Domain}[2i+1]\). |
| `Range` | array | Array of \(2 \times n\) numbers defining output ranges. For each output \(j\), \(\text{Range}[2j] \leq y_j \leq \text{Range}[2j+1]\). |

The stream contains PostScript code enclosed in braces { }. Available operators include:
- **Arithmetic**: abs, add, atan, ceiling, cos, cvi, cvr, div, exp, floor, idiv, ln, log, mod, mul, neg, round, sin, sqrt, sub, truncate
- **Relational/Boolean**: and, bitshift, eq, false, ge, gt, le, lt, ne, not, or, true, xor
- **Conditional**: if, ifelse
- **Stack**: copy, dup, exch, index, pop, roll

Input values form the initial operand stack. After execution, remaining stack values become outputs. The stack must have at least 100 entries. Maximum nesting depth for braces is 255. No composite data types, procedures, variables, or names are allowed.

## Implementation Notes

All function types support clipping to domain and range. The interpolate function is fundamental:

\[
y = \text{Interpolate}(x, x_{\text{min}}, x_{\text{max}}, y_{\text{min}}, y_{\text{max}}) = y_{\text{min}} + \frac{(x - x_{\text{min}}) \times (y_{\text{max}} - y_{\text{min}})}{x_{\text{max}} - x_{\text{min}}}
\]
