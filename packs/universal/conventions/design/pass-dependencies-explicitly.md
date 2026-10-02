---
level: should
decides: design/wiring
option: explicit
review: Flag code that reaches for a global, singleton, or service locator to obtain a collaborator instead of receiving it as a constructor or function parameter.
---
# Pass a component its dependencies; never reach for a global

What a component needs is then visible in its signature, and a test can supply a substitute without patching anything.
