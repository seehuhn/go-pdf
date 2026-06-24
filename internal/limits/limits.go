// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2026  Jochen Voss <voss@seehuhn.de>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

// Package limits collects resource caps used by readers of untrusted
// PDF files.  These caps defend against malicious inputs whose resource
// use is grossly disproportionate to the input file size: decompression
// bombs, allocation bombs, and unbounded recursion.
//
// Most constants cap the number of bytes a particular kind of decoded
// stream may produce; a few cap a derived item count where the byte
// size is not the limiting resource, or a recursion depth where the
// limiting resource is the Go call stack.
package limits

// ImageDataLimit returns an upper bound on the decoded byte count for an
// image with the given parameters.  The bound is
// min(⌈W × channels × bpc / 8⌉ × H, MaxImageBytes).
// It returns MaxImageBytes if any argument is non-positive or if the
// computation overflows.
func ImageDataLimit(width, height, channels, bpc int) int64 {
	if width <= 0 || height <= 0 || channels <= 0 || bpc <= 0 {
		return MaxImageBytes
	}
	size := imageDecodedBytes(width, height, channels, bpc)
	if size < 0 || size > MaxImageBytes {
		return MaxImageBytes
	}
	return size
}

// ImageBytesExceedLimit reports whether an image with the given parameters
// would have a decoded byte count exceeding MaxImageBytes.  It returns
// false if any argument is non-positive; callers must validate those
// separately.
func ImageBytesExceedLimit(width, height, channels, bpc int) bool {
	if width <= 0 || height <= 0 || channels <= 0 || bpc <= 0 {
		return false
	}
	size := imageDecodedBytes(width, height, channels, bpc)
	return size < 0 || size > MaxImageBytes
}

// ImagePixelsExceedLimit reports whether an image with the given pixel
// dimensions exceeds MaxImagePixels.  Unlike [ImageBytesExceedLimit],
// it does not depend on the channel count or bit depth, so it applies
// even when those values are unknown at the dictionary level (e.g. for
// JPXDecode images, where they live in the codestream).  It returns
// false if either argument is non-positive.
func ImagePixelsExceedLimit(width, height int) bool {
	if width <= 0 || height <= 0 {
		return false
	}
	return int64(width)*int64(height) > MaxImagePixels
}

// ImageDecodedFloat64ExceedsLimit reports whether decoding an image with
// the given dimensions into a per-channel float64 buffer would exceed
// MaxImageDecodedFloat64Bytes.  The buffer size is width × height ×
// channels × 8 bytes.  The check guards against amplification: the
// encoded-bytes cap [ImageBytesExceedLimit] allows decoded float64
// buffers up to 64× larger than the encoded stream at bpc=1, so a
// stand-alone cap on the decoded form is needed.  Returns false if any
// argument is non-positive.
func ImageDecodedFloat64ExceedsLimit(width, height, channels int) bool {
	if width <= 0 || height <= 0 || channels <= 0 {
		return false
	}
	// multiply in float64 to avoid integer overflow
	return float64(width)*float64(height)*float64(channels) > MaxImageDecodedFloat64Bytes/8
}

// imageDecodedBytes returns ⌈W × channels × bpc / 8⌉ × H, the decoded
// byte count of an image with the given parameters.  Callers must
// supply pre-bounded inputs (width ≤ [MaxImageWidth], height ≤
// [MaxImageHeight], channels ≤ [MaxImageChannels], bpc ≤ 16); under
// those bounds the product stays well inside int64.
func imageDecodedBytes(width, height, channels, bpc int) int64 {
	bitsPerRow := int64(width) * int64(channels) * int64(bpc)
	bytesPerRow := (bitsPerRow + 7) / 8
	return bytesPerRow * int64(height)
}

const (
	// StreamBudgetBase is the minimum memory budget any stream decode
	// starts with, independent of input size, so that the largest
	// individual filter buffer fits even for tiny inputs.
	StreamBudgetBase = 8 << 20 // 8 MiB

	// StreamBudgetMultiplier is the maximum number of bytes of working
	// memory the filter chain may allocate per byte of raw input.
	StreamBudgetMultiplier = 1024

	// StreamBudgetHardCap caps the input-proportional part of the
	// budget at the same scale as [MaxImageBytes], so a small malicious
	// stream cannot unlock unlimited working memory.
	StreamBudgetHardCap = 256 << 20 // 256 MiB
)

// StreamBudget returns the cumulative memory budget for decoding a
// PDF stream of rawLen on-disk bytes.  The budget is sized as
// [StreamBudgetBase] + min([StreamBudgetMultiplier]·rawLen, [StreamBudgetHardCap])
// so that small streams still get a usable working set while a malicious
// stream cannot unlock unlimited memory through a header claim.
func StreamBudget(rawLen int64) int64 {
	if rawLen < 0 {
		rawLen = 0
	}
	var add int64
	if rawLen > StreamBudgetHardCap/StreamBudgetMultiplier {
		add = StreamBudgetHardCap
	} else {
		add = StreamBudgetMultiplier * rawLen
	}
	return StreamBudgetBase + add
}

// ShadingBudget returns the memory budget for holding the decoded vertices or
// patches of a mesh shading whose stream is rawLen decoded bytes.  The budget
// is sized as [StreamBudgetBase] + [MaxShadingExpansion]·rawLen, so the 8 MiB
// floor admits small meshes regardless of bit packing while the expansion
// slope keeps retained memory proportional to the input.  rawLen is bounded by
// [MaxShadingBytes], so the product stays well inside int64.
func ShadingBudget(rawLen int64) int64 {
	if rawLen < 0 {
		rawLen = 0
	}
	return StreamBudgetBase + MaxShadingExpansion*rawLen
}

const (
	// XRefEntriesBase is the number of cross-reference entries allowed
	// regardless of stream size, so small files with an xref stream are
	// never rejected.
	XRefEntriesBase = 8192

	// XRefEntriesPerByte bounds how many cross-reference entries a stream of
	// a given raw size may declare.  A real xref stream stores high-entropy
	// byte offsets that flate compresses only modestly, so the decoded entry
	// count cannot greatly exceed the compressed size.  The generous factor
	// keeps valid files well clear of the cap while denying the >1000x
	// amplification an all-repeating flate body would otherwise unlock.
	XRefEntriesPerByte = 32
)

// MaxXRefEntries returns the maximum number of entries a cross-reference
// stream of rawLen on-disk bytes may declare.  Decoding allocates one heap
// entry per declared object, so without this bound a tiny FlateDecode body
// could force memory use grossly disproportionate to the input size.  The
// caller is expected to clamp the result against any absolute object-number
// ceiling.
func MaxXRefEntries(rawLen int64) int64 {
	if rawLen < 0 {
		rawLen = 0
	}
	// rawLen is an on-disk byte count bounded by the file size, so the
	// product stays well inside int64
	return XRefEntriesBase + XRefEntriesPerByte*rawLen
}

const (
	// MaxImageWidth and MaxImageHeight are absolute sanity caps on the pixel
	// dimensions of a single image.  Downstream arithmetic uses int64, so
	// this is not an overflow defense; it just bounds resource use.
	MaxImageWidth  = 1 << 16
	MaxImageHeight = 1 << 16

	// MaxImageBytes caps the decoded byte count of a single image
	// XObject, inline image, or thumbnail.
	MaxImageBytes = 256 << 20

	// MaxImageDecodedFloat64Bytes caps the size of a per-channel float64
	// pixel buffer obtained by fully expanding an image's bit-packed
	// samples into floats.  At bpc=1 the expansion ratio over the encoded
	// form is 64×, so the [MaxImageBytes] cap on encoded data is not
	// sufficient.  At 2 GiB this admits 8K UHD CMYK (~1 GiB) with
	// headroom while rejecting clearly malicious amplification.
	MaxImageDecodedFloat64Bytes = 2 << 30

	// MaxImagePixels caps the pixel count (width × height) of a single
	// image.  This bound is independent of channel count and bit depth,
	// so it applies even when those are unknown at extract time (e.g.
	// JPXDecode images, where they live in the JP2 codestream).  At
	// 128 Mpx it covers 8K UHD and most document scans while rejecting
	// the 4 Gpx that the per-axis caps alone would admit.
	MaxImagePixels = 128 << 20

	// MaxImageChannels caps the number of colour components in any image
	// colour space.  All built-in spaces have a fixed channel count
	// (1, 3, or 4); only /DeviceN is open-ended, and no realistic ink
	// set exceeds a handful of components (Hexachrome uses 6, NChannel
	// extends CMYK with a small number of spot colorants).  The cap
	// stops a malicious /DeviceN declaration from amplifying the
	// per-channel float64 buffer that image decoding allocates, and
	// also bounds the soft-mask Matte array (one entry per channel).
	MaxImageChannels = 32

	// MaxSampleBytes caps the decoded byte count of a Type 0 sampled
	// function's sample table.
	MaxSampleBytes = 16 << 20

	// MaxFunctionInputDim caps the number of input dimensions m of a
	// Type 0 sampled function.  Evaluation interpolates over 2^m sample
	// corners, so an unbounded m (admitted by the sample-table cap alone
	// when input dimensions use Size 1) would let a small function consume
	// time exponential in m on every call.  Real sampled functions use
	// m <= 4; this leaves generous headroom.
	MaxFunctionInputDim = 8

	// MaxShadingBytes caps the decoded byte count of a Type 4-7
	// shading stream.
	MaxShadingBytes = 16 << 20

	// MaxShadingExpansion is the per-byte slope of [ShadingBudget]: how many
	// bytes of in-memory vertex/patch storage each decoded stream byte may
	// fund.  A single vertex/patch can be packed into as few as a handful of
	// bits but expands to tens or hundreds of bytes in memory, so the byte cap
	// alone does not bound the allocation.  The factor is set to admit
	// realistic meshes (an 8-bit vertex expands by roughly 14x) with margin,
	// while still keeping retained memory proportional to the input.
	MaxShadingExpansion = 32

	// MaxICCProfileBytes caps the decoded byte count of an ICC color
	// profile stream.
	MaxICCProfileBytes = 32 << 20

	// MaxJBIG2GlobalsBytes caps the decoded byte count of a JBIG2
	// globals stream.
	MaxJBIG2GlobalsBytes = 8 << 20

	// MaxJBIG2PageBytes caps the decoded byte count of a JBIG2
	// per-page stream.  The jbig2 decoder applies its own internal
	// budget on bitmap allocations; this cap bounds only the raw
	// input buffer.
	MaxJBIG2PageBytes = 64 << 20

	// MaxCIDToGIDMapBytes caps the decoded byte count of a font's
	// CIDToGIDMap stream (= 65536 CIDs * 2 bytes/entry).
	MaxCIDToGIDMapBytes = 128 << 10

	// MaxCMapBytes caps the decoded byte count of a CMap or ToUnicode
	// stream.  Realistic CMaps are well under 100 KiB; 4 MiB leaves
	// generous slack for unusually verbose mappings.
	MaxCMapBytes = 4 << 20

	// MaxCMapMappings caps the number of code-to-CID or code-to-Unicode
	// mappings enumerated from a single CMap or ToUnicode CMap.  A wide
	// multi-byte cidrange/bfrange can claim up to 2^32 mappings from a
	// handful of input bytes, so enumerating every code is grossly
	// disproportionate to the input.  The largest predefined CMaps map
	// well under 2^16 codes, so 2^20 leaves generous slack; enumeration
	// stops silently once the bound is reached.
	MaxCMapMappings = 1 << 20

	// MaxFontProgramBytes caps the decoded byte count of an embedded
	// font program (FontFile, FontFile2, FontFile3).  Large TrueType
	// or OpenType fonts can reach several MiB; 16 MiB allows headroom
	// for CJK fonts while bounding decompression amplification.
	MaxFontProgramBytes = 16 << 20

	// MaxStringOrStreamBytes caps the decoded byte count of a value held
	// in the "text string or stream" dual form (form field values, rich
	// text contents, embedded scripts).  Such values carry text, never
	// bulk data, so 16 MiB leaves ample headroom while bounding
	// decompression amplification from the stream form.
	MaxStringOrStreamBytes = 16 << 20

	// MaxIndexedLookupBytes caps the decoded byte count of an Indexed
	// color space lookup table.  PDF 32000-2 §8.6.6.3 bounds the
	// table at (hival+1) * n bytes with hival <= 255 and n <= 32 in
	// any realistic base color space, so 64 KB leaves generous slack.
	MaxIndexedLookupBytes = 64 << 10

	// MaxAlternates caps the number of entries in an image XObject's
	// Alternates array (PDF 32000-2 §8.9.5.4 Table 89).  The spec
	// describes Alternates as a small set of variants of the same
	// image; realistic counts are single-digit.  A list longer than
	// this is treated as malformed and the whole list is dropped, so
	// callers never see a silently truncated set.
	MaxAlternates = 8

	// MaxAssociatedFiles caps the number of entries in an object's AF
	// (associated-files) array (PDF 32000-2 §14.13).  Realistic counts
	// are a handful of attachments per object.  Lists longer than this
	// are dropped wholesale rather than truncated, matching the
	// all-or-nothing semantics of [MaxAlternates].
	MaxAssociatedFiles = 64
)

const (
	// MaxOutlineDepth caps the nesting depth of the document outline
	// tree (PDF 32000-2 §12.3.3), stopping an adversarially deep
	// /First chain before it exhausts the Go call stack.  A
	// legitimately authored outline never approaches this; only a
	// degenerate or malicious chain does.
	MaxOutlineDepth = 256

	// MaxNameTreeDepth caps the nesting depth of a name tree's /Kids
	// chain (PDF 32000-2 §7.9.6), stopping an adversarially deep tree
	// before it exhausts the Go call stack.  A balanced tree stays
	// shallow even for millions of entries, so only a degenerate or
	// malicious chain reaches this.
	MaxNameTreeDepth = 256

	// MaxNumberTreeDepth caps the nesting depth of a number tree's
	// /Kids chain (PDF 32000-2 §7.9.7).  See [MaxNameTreeDepth].
	MaxNumberTreeDepth = 256

	// MaxExtractDepth caps the depth of nested indirect-reference
	// resolution performed by pdf.Decode, stopping an adversarially
	// deep acyclic chain of distinct objects (e.g. Separation-of-Separation
	// color spaces, a long /Next action chain, or a deep field /Kids tree)
	// before it exhausts the Go call stack.  Legitimate composed nesting
	// stays far below this; only a degenerate or malicious chain reaches it.
	MaxExtractDepth = 256
)

const (
	// MaxPageLabelLength caps the number of characters in the numeric
	// portion of a formatted page label (PDF 32000-2 §12.4.2).  The
	// alphabetic styles (/S /a, /S /A) render value n as a single letter
	// repeated ⌈n/26⌉ times, so the output length grows linearly with the
	// attacker-controlled start value /St; without this cap a tiny page
	// label declaration forces a multi-gigabyte string allocation.  Beyond
	// this length the formatter falls back to decimal, which is logarithmic
	// in n.  A label longer than this is never useful for display; 1024
	// leaves vast headroom over any real alphabetic-labelled document.  See
	// [MaxPageLabelStart], which bounds the stored start value on read.
	MaxPageLabelLength = 1024

	// MaxPageLabelStart is the absolute upper bound applied to a page
	// label's /St start value when reading from a file.  No real document
	// numbers pages beyond this, so a larger value is treated as malformed
	// and clamped.  This bounds the integer stored in the data model and
	// serialized across API and IPC boundaries; the actual string
	// allocation is bounded independently by [MaxPageLabelLength].
	MaxPageLabelStart = 1_000_000_000
)
