#! /bin/bash

set -e

go run .

open -a "Google Chrome" test.pdf
sleep 2

open -a "Firefox" test.pdf
sleep 1

open -a "Adobe Acrobat Reader" test.pdf
sleep 3

rm -f test*.png
gs -q -sDEVICE=png16m -r300 -o "test%02d.png" test.pdf
open test*.png
sleep 1

open test.pdf
