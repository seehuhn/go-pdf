#! /bin/bash

echo "finding all *.pdf files on the system ..."

find / \( -name .Trash -o -type l -o -path "/Volumes/*" -o -path "/System/Volumes/*" -o -path "/Users/voss/Library/CloudStorage/*" \) -prune \
    -o -type f -name "*.pdf" -print 2>/dev/null \
| sort \
>all-pdf-files

wc -l all-pdf-files | awk '{ print $1 " PDF files found" }'

echo "done"
