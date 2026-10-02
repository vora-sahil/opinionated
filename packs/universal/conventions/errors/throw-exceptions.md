---
level: should
decides: errors/propagation
option: exceptions
review: Flag functions that signal failure through a return value (null, false, an error code) instead of throwing, and catch blocks that cannot act on what they catch.
---
# Signal failure by throwing, and catch only where you can act on it

Failures then travel to a handler without every caller in between having to pass them along, and none can be ignored by accident.
