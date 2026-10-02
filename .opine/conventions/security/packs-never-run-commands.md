---
level: must
review: Flag any change that lets content from a pack (anything loaded with local=false) reach exec.Command, a shell, or a file write outside .opine/cache.
see: [docs/design.md]
---
# Content from a pack is data: it is never executed and never writes files

A pack is fetched from someone else's repository and runs in CI. If it could run a command, extending a pack would mean handing its author your build machine.
