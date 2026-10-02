---
level: should
decides: testing/doubles
option: fakes
review: Flag tests that assert on which methods were called on a mock when they could assert on the resulting state or output using the real thing or a working fake.
---
# Use the real dependency or a working fake; mock only at the system edge

Tests that check how collaborators were called break on every refactor and pass on real bugs. Tests that check outcomes do neither.
