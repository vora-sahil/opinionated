---
level: should
decides: errors/propagation
option: results
review: Flag expected failures (validation, not found, conflict) signalled by throwing or panicking, and callers that ignore a returned failure.
---
# Return expected failures as values; do not throw for them

The signature then states every way a call can fail, and the caller has to decide what to do about each.
