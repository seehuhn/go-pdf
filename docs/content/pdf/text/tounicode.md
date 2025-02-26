+++
title = 'ToUnicode CMaps'
date = 2024-08-31T22:21:57+01:00
weight = 50
+++

# ToUnicode CMaps

ToUnicode CMaps are used in PDF files to map character codes to Unicode
code points.  They are a special kind of CMap file.

## Structure of a ToUnicode CMap File

A ToUnicode CMap file is a text file (technically, a PostScript source file).
The following information is encoded in the file:

### The CMap name

I have not found any clear information about how to choose the name of a new
ToUnicode CMap.  The ToUnicode CMaps embedded in PDF files on my system use a
wide variety of naming schemes, the most common names being
`/Adobe-Identity-UCS` and `/F7+0`.  In practice, the name probably does not
matter.

Adobe Technical Note #5099: "CMap resource names established by Adobe are
generally composed of up to three parts, each separated by a hyphen. The first
part indicates the character set, the second part indicates the encoding, and
the third part indicates writing direction. Although CMap resources can use
arbitrary names, these conventions make it easier to identify characteristics
of the character set and encoding that they are intended to support."

Adobe Technical Note #5411: "The name of a ToUnicode mapping file consists of
three parts, separated by single hyphens: /Registry string, /Ordering string,
and the /Supplement integer (zero-padded to three digits)."

### A CIDSystemInfo dictionary

I have not found any documentation that describes the CIDSystemInfo dictionary
for a ToUnicode CMap file.  In practice, the value probably does not matter.
The example in the PDF spec uses the following CIDSystemInfo dictionary:

```
/Registry (Adobe)
/Ordering (UCS2)
/Supplement 0
```

In contrast, the vast mojority of PDF files on my laptop uses the followin:

```
/Registry (Adobe)
/Ordering (UCS)
/Supplement 0
```

Related bug report: https://github.com/pdf-association/pdf-issues/issues/344

### The code space ranges

PDF spec: "The CMap file shall contain `begincodespacerange` and `endcodespacerange`
operators that are consistent with the encoding that the font uses. In
particular, for a simple font, the codespace shall be one byte long."

Adobe Technical Note #5411: "Because a ToUnicode mapping file is used to
convert from CIDs (which begin at decimal 0, which is expressed as 0x0000 in
hexadecimal notation) to Unicode code points, the following “codespacerange”
definition, without exception, shall always be used:

```
1 begincodespacerange
<0000> <FFFF>
endcodespacerange
```

I believe the information from Adobe Technical Note #5411 is incorrect, both
the claim that the codespace range is always 0x0000 to 0xFFFF and the claim
that a ToUnicode CMap file maps CIDs (instead of character codes) to Unicode
code points.

### The character mappings

PDF spec: "Use the `beginbfchar`, `endbfchar`, `beginbfrange`, and `endbfrange`
operators to define the mapping from character codes to Unicode character
sequences expressed in UTF-16BE encoding."

Adobe Technical Note #5411: "Lastly, if a CID does not map to a Unicode code
point, the value 0xFFFD (expressed as <FFFD> in the ToUnicode mapping file)
shall be used as its Unicode code point."

## Examples

### Page 358 of ISO 32000-2:2020

```
/CIDSystemInfo <<
  /Registry (Adobe)
  /Ordering (UCS2)
  /Supplement 0
>> def
/CMapName /Adobe-Identity-UCS2 def

1 begincodespacerange
<0000> <FFFF>
endcodespacerange
```

## Adobe Character Collections

ToUnicode CMaps for the standard Adobe Character collections can be found
on GitHub:
- https://github.com/adobe-type-tools/mapping-resources-pdf

## References

- PDF 32000-1:2008 and ISO 32000-2:2020.
  These documents describe ToUnicode CMaps in Section 9.10.3, "ToUnicode CMaps".
  Information about CMap files can also be found in Section 9.7.5.4, "CMap example and operator summary".\
  https://opensource.adobe.com/dc-acrobat-sdk-docs/pdfstandards/PDF32000_2008.pdf#page=301

- Adobe Technical Note #5014, "Adobe CMap and CID Font Files Specification" (June 1993).
  This describes CMap files in general, but has no information specific to ToUnicode CMaps.
  Section 1.6 discusses CMap names.\
  https://adobe-type-tools.github.io/font-tech-notes/pdfs/5014.CIDFont_Spec.pdf

- Adobe Technical Note #5099, "Developing CMap Resources for CID-Keyed Fonts" (March 2012).
  This describes CMap files in general, but has no information specific to ToUnicode CMaps.
  Section 1.6 discusses CMap names.\
  https://adobe-type-tools.github.io/font-tech-notes/pdfs/5099.CMapResources.pdf

- Adobe Technical Note #5411, "ToUnicode Mapping File Tutorial" (May 2003).
  Some of the information in this document seems to be incorrect.\
  https://pdfa.org/norm-refs/5411.ToUnicode.pdf

- "PostScript Language Reference", third edition.
  This describes CMap files in general, but has no information specific to ToUnicode CMaps.
  Particularly relevant is Section 5.11.4, "CMap Dictionaries".
  https://www.adobe.com/jp/print/postscript/pdfs/PLRM.pdf#page=396

- https://github.com/pdf-association/pdf-issues/issues/344
