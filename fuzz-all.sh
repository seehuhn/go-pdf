#! /bin/bash

set -e

fuzz_time=30s
while getopts "t:" opt; do
    case "$opt" in
        t)
            fuzz_time="$OPTARG"
            ;;
        *)
            echo "usage: fuzz-all.sh [-t 30s]" 1>&2
            exit 1
            ;;
    esac
done
shift $((OPTIND-1))

ok=true
while IFS=":" read -r dir_name test_name; do
    if ! grep -q "$test_name" "$dir_name"/*_test.go; then
        echo "$dir_name/testdata/fuzz/$test_name is unused" 1>&2
        ok=false
    fi
done < <(find . -path "*/testdata/fuzz/*" -type d \
        | sed -e 's;^\(.*\)/testdata/fuzz/\(.*\);\1:\2;')
if [ "$ok" = false ]; then
    exit 1
fi

find . -name "*_test.go" -print0 \
| xargs -0 grep -H '^func Fuzz' \
| sed -e 's;^\(.*\)/[^/]*:func \(Fuzz[A-Za-z0-9_]*\).*$;\1:\2;' \
| shuf \
| while IFS=":" read -r file_name test_name; do
    echo ""
    echo "# $file_name $test_name"
    go test -fuzz="^$test_name\$" "$file_name" -fuzztime="$fuzz_time"
done
