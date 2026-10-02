---
level: must
paths: ["**"]
check:
  kind: forbid
  pattern: '^(<<<<<<<|>>>>>>>) '
emit: false
---
# Never commit a merge conflict marker

A marker means a merge was never finished. The file is usually broken and the intended code is ambiguous.
