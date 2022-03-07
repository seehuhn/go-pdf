#! /bin/bash

set -e

ok=true
while IFS=":" read -r dir_name test_name; do
    if ! grep -q "$test_name" "$dir_name"/*_test.go; then
        echo "$dir_name/testdata/fuzz/$test_name is unused"
        ok=false
    fi
done < <(find . -path "*/testdata/fuzz/*" -type d \
        | sed -e 's;^\(.*\)/testdata/fuzz/\(.*\);\1:\2;' )
if [ "$ok" = false ]; then
    exit 1
fi

find . -name "*_test.go" -print0 \
| xargs -0 grep '^func Fuzz' \
| sed -e 's;^\(.*\)/[^/]*:func \(Fuzz[A-Za-z0-9]*\).*$;\1:\2;' \
| shuf \
| while IFS=":" read -r file_name test_name; do
    echo ""
    echo "# $file_name $test_name"
    go1.18rc1 test -fuzz="$test_name" "$file_name" -fuzztime=1m
done
