+++
title = 'Operators'
date = 2024-09-15T12:46:50+01:00
draft = true
+++

# Operators in a PDF file

## PDF Keywords

- In PDF, white space characters are:
    | ASCII | Name |
    |:-----:|:----:|
    | 0     | NUL  |
    | 9     | HT   |
    | 10    | LF   |
    | 12    | FF   |
    | 13    | CR   |
    | 32    | SP   |
- Delimiters are:
    | ASCII | Name |
    |:-----:|:----:|
    | 40    | (    |
    | 41    | )    |
    | 60    | <    |
    | 62    | >    |
    | 91    | [    |
    | 93    | ]    |
    | 123   | {    |
    | 125   | }    |
    | 47    | /    |
    | 37    | %    |
- All characters except the white-space characters and delimiters are referred
  to as regular characters. These characters include bytes that are outside the
  ASCII character set.  A sequence of consecutive regular characters comprises
  a single token \[Section 7.2.3\].
- No `#` escapes are used in PDF keywords.
- https://github.com/pdf-association/pdf-issues/issues/363

## Content Stream Operators

## PostScript Operators

- All characters besides the white-space characters and delimiters are referred
  to as regular characters. These include nonprinting characters that are
  outside the recommended PostScript ASCII character set
- Any token that consists entirely of regular characters and cannot be
  interpreted as a number is treated as a name object (more precisely, an
  executable name).  All characters except delimiters and white-space
  characters can appear in names, including characters ordinarily considered to
  be punctuation.
