Color Spaces
============

The purpose of PDF color spaces is to tell the output device how to interpret
the color data in the PDF file.


Device Color Spaces
-------------------

The device color spaces are:

- DeviceGray (PDF-1.1)
  Color space description: \DeviceGray

- DeviceRGB (PDF-1.1)
  Color space description: \DeviceRGB

- DeviceCMYK (PDF-1.1)
  Color space description: \DeviceCMYK

- CalCMYK is a deprecated synonym for DeviceCMYK.
  Color space description: [\CalCMYK dict]

The device color spaces can have two different uses:

When used directly, they give colour values correct for a specific output
device. When shown on a different device, the colours may not be accurate.
Alternatively, these colour spaces can be used with the DefaultRGB, DefaultCMYK
and DefaultGray colour spaces to provide device independent colour.


CIE-Based Color Spaces
----------------------

The CIE-based color spaces are:

- CalGray (PDF-1.1)
  Color space description: [\CalGray dict]
  The dictionary entries are \WhitePoint (required), \BlackPoint (optional)
  and \Gamma (optional).
  See table 62 in ISO 32000-2:2020.

- CalRGB (PDF-1.1)
  Color space description: [\CalRGB dict]
  The dictionary entries are \WhitePoint (required), \BlackPoint (optional),
  \Gamma (optional) and \Matrix (optional).
  See table 63 in ISO 32000-2:2020.

- Lab (PDF-1.1)
  Color space description: [\Lab dict]
  The dictionary entries are \WhitePoint (required), \BlackPoint (optional)
  and \Range (optional).
  See table 64 in ISO 32000-2:2020.

- ICCBased (PDF-1.3)
  Color space description: [\ICCBased stream]
  The stream dictionary has the following (additional) entries: \N (required),
  \Alternate (deprecated), \Range (optional), and \Metadata (optional).
  See table 65 in ISO 32000-2:2020.

Special Color Spaces
--------------------

The special color spaces are:

- Indexed (PDF-1.1)
  Color space description: [\Indexed baseColor highVal lookup]
  See section 8.6.6.3 in ISO 32000-2:2020.

- Pattern (PDF-1.2)
  Colored patterns: /Pattern
  Uncolored tiling patterns: [/Pattern baseColorSpace]

- Separation (PDF-1.2)
  Color space description: [\Separation name alternateSpace tintTransform]
  See section 8.6.6.4 in ISO 32000-2:2020.

- DeviceN (PDF-1.3)
  Color space description: [\DeviceN names alternateSpace tintTransform]
      or [/DeviceN names alternateSpace tintTransform attributes]
  See section 8.6.6.5 in ISO 32000-2:2020.


ICC Profiles
============

The following list shows the maximum ICC profile version that can be used with
each PDF version:

PDF-1.3
    profile version 2.1.0, spec version ICC 3.3 (Nov 1996)

PDF-1.4
    profile version 2.3.0, spec version ICC.1A:1999-04

PDF-1.5
    profile version 4.0.0, spec version ICC.1:2001-12

PDF-1.6
    profile version 4.1.0, spec version ICC.1:2003-09

PDF-1.7 and PDF-2.0
    profile version 4.3.0, spec version ICC.1:2010-12 (identical to ISO 15076-1:2010)


Resources
=========

https://github.com/lucasb-eyer/go-colorful
https://developer.apple.com/library/archive/technotes/tn2313/_index.html
https://en.wikipedia.org/wiki/CIE_1931_color_space

https://medium.com/hipster-color-science/a-beginners-guide-to-colorimetry-401f1830b65a

https://www.color.org/specification/ICC1v43_2010-12.pdf
