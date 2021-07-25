#! /bin/bash
for name in ../../font/truetype/ttf/*.ttf; do
    go run . "$name"
    mv test.pdf "$(basename "$name").pdf"
done
