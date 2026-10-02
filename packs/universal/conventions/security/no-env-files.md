---
level: must
check:
  kind: absent
  files: ["**/.env", "**/.env.local", "**/.env.production"]
emit: false
---
# Keep .env files out of the repository

They hold real credentials. Commit a `.env.example` with placeholder values and ignore the real one.
