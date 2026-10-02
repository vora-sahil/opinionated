---
paths:
  - "internal/**/*.go"
  - "cmd/**/*.go"
---
# Print only through the command's writers, never straight to stdout or stderr

Every command is run in tests with its output captured. A direct print bypasses that, so the behaviour it produces cannot be asserted.

Bad:
```go
fmt.Println("No violations.")
```

Good:
```go
e.out("No violations.\n")
```

_must · `style/output-through-env` · checked by `opine check`_
