The code in this directory creates a test page which draws the same
16x16 1-bit image behind many different /Filter chains: chains of
increasing length without repeated filters (section A), short chains
which use one filter twice (section B), and long chains cycling through
the five general-purpose lossless filters (section C).

The PDF spec sets no limit on the length of a filter chain.  Table 5
(the Filter entry of the stream dictionary) only notes that including
the same filter more than once "is not recommended".

Results (October 2026): no viewer tested enforces a chain length
limit, but some viewers show a blank image for a chain which repeats a
decompression filter.  Repeated ASCII filters render.  Quire renders
all chains.
