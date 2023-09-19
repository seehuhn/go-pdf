# Notes about ToUnicode CMaps

ToUnicode CMaps are used in PDF files to map character codes to Unicode
code points.  They are a special kind of CMap file.

## Structure of a ToUnicode CMap File

A ToUnicode CMap file is a text file (technically, a PostScript source file).
The following information encoded in the file:

### The CMap name

Adobe Technical Note #5099: "CMap resource names established by Adobe are
generally composed of up to three parts, each separated by a hyphen. The first
part indicates the character set, the second part indicates the encoding, and
the third part indicates writing direction. Although CMap resources can use
arbitrary names, these conventions make it easier to identify characteristics
of the character set and encoding that they are intended to support."

Adobe Technical Note #5411: "The name of a ToUnicode mapping file consists of
three parts, separated by single hyphens: /Registry string, /Ordering string,
and the /Supplement integer (zero-padded to three digits)."

I believe that the information from Adobe Technical Note #5411 is incorrect
here.  For example, Adobe uses the name `Adobe-Identity-UCS2` for one of
their cmaps, but the name does not follow the pattern described above.

### A CIDSystemInfo dictionary

The example in the PDF spec uses the following CIDSystemInfo dictionary:

```
/Registry (Adobe)
/Ordering (UCS2)
/Supplement 0
```

### The code space ranges

PDF spec: "Use the beginbfchar, endbfchar, beginbfrange, and endbfrange
operators to define the mapping from character codes to Unicode character
sequences expressed in UTF-16BE encoding."

PDF spec: "The CMap file shall contain begincodespacerange and endcodespacerange
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

I believe the information from Adobe Technical Note #5411 is incorrect,
both the claim that the codespace range is always 0x0000 to 0xFFFF and
the claim that a ToUnicode CMap file maps CIDs (instead of character codes) to Unicode code points.

### The character mappings

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

## References

- PDF 32000-1:2008 and ISO 32000-2:2020.
  These documents describe ToUnicode CMaps in Section 9.10.3, "ToUnicode CMaps".
  Information about CMap files can also be found in Section 9.7.5.4, "CMap example and operator summary".

- Adobe Technical Note #5014, "Adobe CMap and CID Font Files Specification" (June 1993).
  This describes CMap files in general, but has no information specific to ToUnicode CMaps.
  Section 1.6 discusses CMap names.
  https://adobe-type-tools.github.io/font-tech-notes/pdfs/5014.CIDFont_Spec.pdf

- Adobe Technical Note #5099, "Developing CMap Resources for CID-Keyed Fonts" (March 2012).
  This describes CMap files in general, but has no information specific to ToUnicode CMaps.
  Section 1.6 discusses CMap names.
  https://adobe-type-tools.github.io/font-tech-notes/pdfs/5099.CMapResources.pdf

- Adobe Technical Note #5411, "ToUnicode Mapping File Tutorial" (May 2003).
  Some of the information in this document seems to be incorrect.
  https://www.adobe.com/content/dam/acom/en/devnet/acrobat/pdfs/5411.ToUnicode.pdf

- "PostScript® LANGUAGE REFERENCE", third edition.
  This describes CMap files in general, but has no information specific to ToUnicode CMaps.
  Particularly relevant is Section 5.11.4, "CMap Dictionaries".
