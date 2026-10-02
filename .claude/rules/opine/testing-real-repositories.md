---
paths:
  - "internal/**/*_test.go"
  - "packs/**/*_test.go"
---
# Test against a real temporary repository, not a stubbed filesystem

opine's whole job is reading files and directories. A test that fakes them proves nothing about globs, git, or path handling, which is where the bugs are.

Good:
```go
root := testrepo.New(t, map[string]string{
	".opine/opine.yaml": "version: 1\n",
	"src/a.kt":          "try { x() } catch (e: E) { }\n",
})
```

_should · `testing/real-repositories` · checked in review: Flag tests that stub the filesystem or the pack loader instead of writing files with testrepo.New and running the real code against them._
