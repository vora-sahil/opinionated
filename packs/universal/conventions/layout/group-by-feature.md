---
level: should
decides: layout/organization
option: by-feature
review: Flag new top-level directories named after a technical layer (controllers, services, models, utils) and features whose files are spread across several of them.
---
# Group code by feature, not by technical layer

A change usually touches one feature. Keeping its handler, logic, and storage together makes that change one directory, and makes a feature deletable.
