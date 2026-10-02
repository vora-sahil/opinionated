---
level: must
paths: ["internal/**/*.go", "cmd/**/*.go"]
exclude: ["**/*_test.go"]
check:
  kind: forbid
  pattern: 'fmt\.Print(f|ln)?\(|os\.Std(out|err)\.Write'
  message: Write through the command's env (e.out, e.errf), not directly to the process streams
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
