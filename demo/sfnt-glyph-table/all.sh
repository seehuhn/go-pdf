#! /bin/bash
for name in ../../sfnt/ttf/*.ttf ../../sfnt/otf/*.otf; do
    echo "$name"
    go run . "$name" && mv test.pdf "$(basename "$name").pdf"
done
