This example tests where a viewer places the "current point" of a PDF
graphics path, after `closepath` has been called.  The code draws
two closed shapes, followed by an extra line.  The starting point
of the extra line, if any, is the current point after the
first part of the path has been closed.
