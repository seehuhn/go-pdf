#! /bin/bash

set -e

find . -name "*_test.go" -print0 \
| xargs -0 grep '^func Fuzz' \
| sed -e 's;^\(.*\)/[^/]*:func \(Fuzz[A-Za-z0-9]*\).*$;\1:\2;' \
| sort \
| while IFS=":" read -r file_name test_name; do
    echo ""
    echo "# $file_name $test_name"
    go1.18rc1 test -fuzz="$test_name" "$file_name" -fuzztime=1m
done
