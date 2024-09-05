+++
title = 'Stream Filters'
date = 2024-09-05T10:53:38+01:00
draft = true
+++

# Stream Filters

PDF supports 10 different types of filters which can be used to compress,
format or encrypt PDF streams:

- `ASCIIHexDecode`: encode bytes using hexadecimal notation
- `ASCII85Decode`: encode bytes using the ASCII85 format
- `LZWDecode`: compress bytes using the LZW algorithm
- `FlateDecode`: (PDF 1.2) compress bytes using the zlib/deflate algorithm
- `RunLengthDecode`: compress bytes using simple run-length encoding
- `CCITTFaxDecode`: compress image data using the CCITT Fax standard
- `JBIG2Decode`: (PDF 1.4) compress image data using the JBIG2 standard
- `DCTDecode`: compress image data using the JPEG standard
- `JPXDecode`: (PDF 1.5) compress image data using the JPEG2000 standard
- `Crypt`: (PDF 1.5) encrypt the data of this stream
