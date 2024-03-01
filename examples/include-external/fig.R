#! /usr/bin/env Rscript

pdf("fig.pdf", 6, 6)
pairs(iris[, 1:4], col = as.integer(iris$Species) + 1)
invisible(dev.off())
