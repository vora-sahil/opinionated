---
level: should
decides: api/surface
option: entry-point
review: Flag imports that reach into another module's internal files instead of its entry point (index, __init__, mod, or public package).
---
# Import another module only through its entry point

Everything behind the entry point can then be reorganised freely, because nothing outside depends on where it lives.
