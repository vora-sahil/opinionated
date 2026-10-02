---
level: must
paths: ["**"]
check:
  kind: forbid
  pattern: '-----BEGIN [A-Z ]*PRIVATE KEY-----|\bAKIA[0-9A-Z]{16}\b|\bgh[pousr]_[A-Za-z0-9]{36}\b|\bxox[baprs]-[A-Za-z0-9-]{10,}|\bsk_live_[A-Za-z0-9]{16,}'
  message: This looks like a credential. Remove it, rotate it, and load it from the environment instead
---
# Never commit a secret

A secret in git history is compromised for good: deleting the line does not delete the commit. Rotating the credential is the only fix.
