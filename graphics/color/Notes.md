Color Spaces
============

The purpose of PDF color spaces is to tell the output device how to interpret
the color data in the PDF file.


Device Color Spaces
-------------------

The device color spaces are:

- DeviceGray (PDF-1.1)
- DeviceRGB (PDF-1.1)
- DeviceCMYK (PDF-1.1)

- CalCMYK is a synonym for DeviceCMYK.

The device color spaces can have two different uses:

When used directly, they give colour values correct for a specific output
device. When shown on a different device, the colours may not be accurate.
Alternatively, these colour spaces can be used with the DefaultRGB, DefaultCMYK
and DefaultGray colour spaces to provide device independent colour.


CIE-Based Color Spaces
----------------------

The CIE-based color spaces are:

- CalGray (PDF-1.1)
- CalRGB (PDF-1.1)
- Lab (PDF-1.1)
- ICCBased (PDF-1.3)


Special Color Spaces
--------------------

The special color spaces are:

- Indexed (PDF-1.1)
- Pattern (PDF-1.2)
- Separation (PDF-1.2)
- DeviceN (PDF-1.3)



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
