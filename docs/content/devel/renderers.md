+++
title = 'Renderers'
date = 2024-09-04T16:42:51+01:00
draft = true
+++

Here is a list of PDF renderers that I sometimes try on my Mac:

- MacOS Preview: `open test.pdf`
- Acrobat Reader: `open -a "Adobe Acrobat Reader" test.pdf`
- Google Chrome: `open -a "Google Chrome" test.pdf`
- Firefox: `open -a "Firefox" test.pdf`
- GhostScript: `gs -q -sDEVICE=png16m -r300 -o test.png test.pdf && open test.png`
