+++
title = 'CID Values'
date = 2025-02-26T15:01:41Z
+++

# CID Values

Considerations for allocating CID values:

- There must be a on-to-one mapping between CID and GID values.
- The total number of CIDs affects file size for embedded TrueType fonts.
- If CID=GID, simple CFF fonts can be used.
- If consecutive CIDs correspond to consecutive GIDs,
  this reduces file size for CID-keyed CFF fonts.
- predefined CID systems are required for non-embedded TrueType fonts.
- predefined CID systems allow for smaller/missing ToUnicode CMaps.
- encoding may be more convenient if CID values are related to text content.

Some ideas for allocating CID values:

+ **Use CID defined in the source font**
  - Not always available
  - Good for non-embedded fonts

+ **Use CID = GID**
  - After subsetting, this will no longer be the identity mapping.

+ **Use the CID system from one of the predefined CMaps.**
  - Large number of CID values.
  - For embedded/subsetted fonts, a non-trivial mapping from CID to GID is
    required.
  - Text content is implied

+ **Allocate CID values sequentially.**
    Smallest possible number of CID values.
    Can use an identity CID to GID mapping.

    If human-readable text strings are required, the CMap file
    may be large, because consecutive codes will not correspond
    to consecutive CID values.

+ **Allocate CIDs based on text content.**
    This can potentially lead to a large number of CID values.

+ **Hybrid approach.**
    I could use CID=ASCII-31 for ASCII characters, and sequential
    allocation, starting at CID 128 for everything else.


# Character Codes

Consideration for allocating character codes:

- Shorter codes lead to smaller text strings.
- Predefined CMaps don't need to be embedded.
- Predefined CMaps different from the identity ones imply the CID system.
- If consecutive codes correspond to consecutive CIDs, this reduces CMap file
  size.
- A human-readable encoding like utf-8 is convenient for debugging.
- MacOS does not support codes longer than 2 bytes
