---
level: should
decides: workflow/todos
option: tracked
paths: ["**"]
exclude: ["**/*.md"]
check:
  kind: forbid
  pattern: '(//|#|/\*|^\s*\*|--|<!--)\s*(TODO|FIXME)\b'
  unless: '\b(TODO|FIXME)\([^)]+\)|#\d+|https?://|\b[A-Z][A-Z0-9]+-\d+\b'
  message: 'Give this TODO an owner or an issue, e.g. TODO(name) or TODO #123'
---
# Give every TODO an owner or an issue

An untracked TODO is a wish. One with a name or a ticket is work someone can find, schedule, and close.
