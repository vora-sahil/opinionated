---
paths:
  - "internal/**/*.go"
---
# End every error message with what to do about it

Bad:
```go
add("unknown topic %q", topic)
```

Good:
```go
add("unknown topic %q; topics are: %s", topic, strings.Join(Topics, ", "))
```

_should · `docs/messages-say-what-to-do` · checked in review: Flag error and problem messages that state only what is wrong. Each should name the valid values, the fix, or the command to run next._
