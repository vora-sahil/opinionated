---
level: should
review: When a manifest or lockfile gains a dependency, the change description must say what it is for and why existing code or dependencies do not cover it.
---
# Justify every new third-party dependency in the change that adds it

A dependency is code you maintain without having written it: upgrades, licences, and security advisories, for as long as it stays.
