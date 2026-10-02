---
level: should
review: Flag empty catch or except blocks, ignored error return values, and catch-alls that neither log, rethrow, nor return a failure.
---
# Never discard an error silently

A swallowed error turns a clear failure at the cause into a confusing one somewhere else, later.
