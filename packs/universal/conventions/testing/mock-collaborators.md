---
level: should
decides: testing/doubles
option: mocks
review: Flag unit tests that exercise real collaborators (database, network, other modules) instead of mocking them.
---
# Mock every collaborator so a unit test exercises one unit

A failing test then points at exactly one unit, and the suite stays fast because nothing real is started.
