---
level: should
decides: workflow/todos
option: forbidden
paths: ["**"]
exclude: ["**/*.md"]
check:
  kind: forbid
  pattern: '(//|#|/\*|^\s*\*|--|<!--)\s*(TODO|FIXME)\b'
  message: Do the work or file an issue; TODOs are not merged here
---
# Do not merge a TODO

Deferred work belongs in the issue tracker, where it is visible and prioritised. In the code it only decays.
