+++
title = 'Coordinate Systems'
date = 2024-10-08T09:11:34+01:00
+++

# Coordinate Systems

PDF uses various coordinate systems.

## Device Space

## User Space

## Image Space

## Form Space

## Pattern Space

## Text Space

## PDF Glyph Space

PDF glyph coordinates can be different from the font's glyph coordinates. For
all font types except Type 3, the units of PDF glyph space are one-thousandth
of a unit of text space.  For Type 3 fonts, the transformation from glyph space
to text space is defined by the font matrix specified in the `FontMatrix` entry
in the font dict.  This is explained in Section 9.2.4 of ISO 32000-2:2020.

See [PDF issue #378](https://github.com/pdf-association/pdf-issues/issues/378)
for additional discussion.
