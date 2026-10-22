The trailer /Size entry is defined as one greater than the highest object
number in the file, and the specification says that any object in a
cross-reference section whose number is greater than /Size shall be treated
as missing.  Writers get /Size wrong often enough that readers may prefer to
ignore it.  This probe shows which choice a viewer makes.

The file "test.pdf" is a single A4 page, built by hand.  Its cross-reference
table lists ten objects, but the trailer says /Size 7.  Each of the two
tests depends on one object at or beyond /Size.  Both objects are the
values of optional entries, and a null value for an optional entry means
the same as an absent one, so a reader which drops either object violates
nothing.

Test A: object 7, numbered exactly /Size, is the /OFF array of the optional
content default configuration.  The page prints "yes" and then paints "no"
over it inside the optional content group.  A reader which resolves the
array switches the group off and reveals "yes".  The specification is
inconsistent at this boundary: by the definition of /Size no object 7 can
exist, but the rule for ignoring entries only covers numbers greater than
/Size.  The result is only meaningful if test B says "yes".

Test B: object 8, above /Size, is the page's /Annots array.  The page
prints "yes"; a reader which resolves the array draws the annotation it
lists, whose appearance paints "no" over the answer.

Results so far: go-pdf, Poppler and Ghostscript all ignore /Size, showing
"yes" for test A and "no" for test B.
