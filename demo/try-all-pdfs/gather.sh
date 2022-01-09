#! /bin/bash

echo "finding all *.pdf files on the system ..."

find / \( -name .Trash -o -type l -o -path "/Volumes/*" -o -path "/System/Volumes/*" \) -prune \
    -o -type f -name "*.pdf" -print 2>/dev/null \
| sort \
>all-fonts

wc -l all-fonts | awk '{ print $1 " PDF files found" }'

echo "done"
