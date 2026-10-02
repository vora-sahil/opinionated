# opinionated

**Keep a codebase's conventions written down, enforced, and in front of every agent and person who works on it.**

Every good codebase has opinions: where things go, how errors travel, what a test looks like, which module may import which. They used to live in the heads of the people who wrote the code and in the code itself. As agents write more of it, those opinions are either forgotten or quietly averaged away.

`opine` is the thing a staff engineer would set up on day one, made into a tool:

- **One short file per convention.** The rule in one sentence, the reason, an example, and how it is checked. Short enough that a person can read it and say "yes, that is how we do it here."
- **Every convention is enforced**, by a machine check where one is possible and by a reviewer's instruction where it is not. A convention with neither is rejected.
- **Agents are told only what they need**, in the files they already read (`AGENTS.md`, `CLAUDE.md`, path-scoped rules), generated from the same source.
- **It starts from your code.** On an existing repository it proposes what the code already does and asks you to confirm. On a new one it gives you a baseline.
- **Opinions are portable.** A personal or company pack is a git repository; a repo extends it and records, with a reason, wherever it deviates.

It is one static binary with no runtime, works on any language, and makes no model calls of its own.

## What a convention looks like

`.opine/conventions/boundaries/database-only-in-core.md`:

````markdown
---
level: must
paths: ["apps/**/*.ts"]
check:
  kind: forbid
  pattern: 'from "better-sqlite3"'
---
# Only packages/core opens the database; apps go through its exports

One module owning the connection keeps migrations and pragmas in one place.

Bad:
```ts
import Database from "better-sqlite3";
```

Good:
```ts
import { getStoryView } from "@braid/core";
```
````

That file is the whole convention. From it:

```console
$ opine check
apps/api/src/raw.ts
     1  must    boundaries/database-only-in-core    Only packages/core opens the database; apps go through its exports

1 violation: 1 must, 0 should.
```

and an agent editing anything under `apps/` is shown the rule, the reason and the example, while one editing elsewhere is not shown it at all.

## Install

```console
$ go install github.com/vora-sahil/opinionated/cmd/opine@latest
```

## Start

**In an existing repository:**

```console
$ opine init                      # reads the repo; proposes what it already does
$ opine prompt mine               # optional: instructions for your agent to propose the rest
$ opine review                    # yes or no to each proposal, one at a time
$ opine check --update-baseline   # grandfather today's violations; hold the line from here
$ opine emit                      # write AGENTS.md, CLAUDE.md, CONVENTIONS.md
```

`opine init` only counts: it proposes a naming convention when nearly every file already follows it, and shows the numbers. `opine prompt mine` prints instructions any coding agent can follow to propose the conventions that need judgment. Nothing proposed is in force until you accept it.

**In a new repository:** `opine init` gives you the baseline: a small set of conventions that hold in any language, and a default answer to each decision that has one.

**Then, every day:**

```console
$ opine check          # in CI; fails on any 'must'
$ opine emit --check   # in CI; fails if the agent files are out of date
```

To give an agent feedback the moment it breaks a convention, add this to `.claude/settings.json`:

```json
{
  "hooks": {
    "PostToolUse": [
      { "matcher": "Edit|Write", "hooks": [{ "type": "command", "command": "opine check --hook" }] }
    ]
  }
}
```

## The pieces

| | What it is | Where |
|---|---|---|
| **Convention** | One rule, its reason, an example, and its check | `.opine/conventions/<topic>/<name>.md` |
| **Decision** | A question every codebase answers (how do failures propagate? where do tests live?) with named options. You pick one, in one line | `decisions:` in `.opine/opine.yaml` |
| **Structure** | The folder tree, what each folder is for, and which may import which | `.opine/structure.yaml` |
| **Pack** | A set of conventions and decisions to inherit: built in, a path, or a git repository pinned to a commit | `extends:` in `.opine/opine.yaml` |
| **Override** | A recorded, reasoned deviation from an inherited convention | `overrides:` in `.opine/opine.yaml` |
| **Baseline** | The violations that existed when a convention was adopted. Nothing may get worse | `.opine/baseline.json` |

[docs/format.md](docs/format.md) is the full reference. [docs/design.md](docs/design.md) explains why each piece is the way it is, and what it was learned from.

This repository uses opine on itself: see [CONVENTIONS.md](CONVENTIONS.md) and [.opine/](.opine).

## Where this stands

Working today:

- The convention format and its validation (`opine lint`)
- Eight language-agnostic check kinds, inline suppressions with mandatory reasons, and the baseline
- Folder structure: shape, naming, layers and isolation, with import resolution for JavaScript and TypeScript, Python, Go, Java, Kotlin, Scala, Rust, C#, PHP, Ruby and C/C++
- Packs from a path or a pinned git repository, the cascade, decisions and overrides
- `emit` for AGENTS.md, Claude Code, Cursor and GitHub Copilot, and the CONVENTIONS.md digest
- `init` inference, `prompt mine`, `prompt review`, `review`, `explain`
- JSON and SARIF output, and an agent hook mode

Not built yet:

- **Language and framework packs.** Only the universal pack ships. Deep packs (TypeScript and React, Python, Kotlin and the JVM, Go, Rust and others) will wrap each ecosystem's own linters rather than restate them.
- **Native adapters**, so a pack can configure eslint, ruff, detekt and the like. Today a repository can wrap any tool with a `run` check; a pack cannot, by design.
- **`opine drift`**: flag conventions the code has stopped following and checks that never fire.
- **Carrying a personal pack into a repository you do not own**: proposing your conventions to a team, and emitting them to your own user-level agent configuration.
- **Packs that extend other packs.**

## Licence

MIT
