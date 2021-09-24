#! /bin/bash

echo "finding all *.ttf and *.otf fonts on the system ..."

find / \( -name .Trash -o -type l \) -prune \
    -o -type f \( -name "*.ttf" -o -name "*.otf" \) -print 2>/dev/null \
| sort \
>all-fonts

wc -l all-fonts | awk '{ print $1 " fonts found" }'

echo "done"
