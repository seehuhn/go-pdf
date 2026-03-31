#! /bin/bash
for name in ../../../ttf/*.ttf ../../../otf/*.otf; do
    echo "$name"
    go run . "$name" && mv test.pdf "$(basename "$name").pdf"
done
