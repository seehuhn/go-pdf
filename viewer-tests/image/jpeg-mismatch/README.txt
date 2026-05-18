The code in this directory creates a test page with a JPEG image.
The image dictionary in the PDF file specifies a 32x32 image,
but the embedded JPEG file providing the image data is 64x64.
The PDF spec seems to not specify what should happen in this case.

Results (May 2026):

* Some readers ignore the intrinsic image dimensions of the JPEG file
  and just use (the start of) the byte stream to fill a 32x32 image.
  Examples: Adobe Acrobat Reader, Ghostscript

* Some readers use the intrinsic image dimensions of the JPEG file,
  ignoring the dimensions specified in the PDF image dictionary.
  Examples: Apple Preview, Firefox

* Some readers use the intrinsic image dimensions of the JPEG file,
  and then crop the image to the dimensions specified in the PDF image dictionary.
  Examples: Google Chrome, PDF Studio Viewer 2024
