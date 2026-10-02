---
level: should
decides: design/wiring
option: container
review: Flag components that construct their own collaborators or read globals instead of declaring them for the container to inject.
---
# Declare dependencies and let the container provide them

Wiring lives in one place, so lifetimes and substitutions are changed there rather than at every construction site.
