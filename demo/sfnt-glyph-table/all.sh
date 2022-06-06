#! /bin/bash
for name in ../../font/ttf/*.ttf; do
    echo "$name"
    go run . "$name"
    mv test.pdf "$(basename "$name").pdf"
done
