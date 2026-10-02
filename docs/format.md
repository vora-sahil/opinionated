# Format reference

Everything opine reads lives in `.opine/` at the repository root.

```text
.opine/
  opine.yaml          configuration: packs, decisions, overrides
  conventions/        one markdown file per convention
    <topic>/<name>.md
  structure.yaml      the folder layout (optional)
  decisions.yaml      decisions this repository defines itself (optional)
  baseline.json       written by 'opine check --update-baseline'
  opine.lock          written by 'opine sync'; commit it
  cache/              fetched git packs; not committed
```

## Conventions

A convention is a markdown file. Its path is its identity: `.opine/conventions/errors/no-empty-catch.md` has the id `errors/no-empty-catch` and the topic `errors`.

````markdown
---
level: must
paths: ["src/**/*.kt"]
check:
  kind: forbid
  pattern: 'catch \([^)]*\)\s*\{\s*\}'
---
# Never leave a catch block empty

Silent failures hid the billing outage for six hours.

Bad:
```kotlin
try { charge() } catch (e: Exception) {}
```

Good:
```kotlin
try { charge() } catch (e: Exception) { log.error("charge failed", e); throw e }
```
````

### Body

| Part | How it is written | Limit |
|---|---|---|
| Rule | The `# ` heading: one imperative sentence | 100 characters |
| Reason | The paragraph under the heading | 300 characters |
| Examples | A line reading `Bad:` or `Good:`, then a fenced code block | 2 of each, 15 lines each |
| Whole body | | 40 lines |

The limits are part of the format and `opine lint` enforces them. They exist so that a convention can be read and judged in a few seconds. Reasoning that needs more room goes in a document linked with `see:`.

### Frontmatter

| Field | Values | Default | Meaning |
|---|---|---|---|
| `level` | `must`, `should`, `off` | `should` | `must` fails `opine check`. `should` is reported but does not fail it. |
| `status` | `proposed`, `accepted`, `rejected`, `deprecated` | `accepted` | Only accepted conventions are in force. |
| `paths` | list of globs | whole repository | Files the convention covers. |
| `exclude` | list of globs | | Files it does not cover. |
| `check` | see below | | How a machine enforces it. |
| `review` | one sentence | | What a reviewer should flag. |
| `emit` | `true`, `false` | `true` | `false` keeps it out of agent files. Use it when a check alone is enough. |
| `decides`, `option` | a decision id and an option name | | Makes this convention one option of a decision. |
| `see` | list of paths or URLs | | Longer reasoning: an ADR, an incident report. |
| `tags` | list of strings | | Free-form. |
| `evidence` | `follows`, `violates`, `examples` | | On a proposal: how true it already is. |
| `reason` | text | | Required when status is `rejected` or `deprecated`. |

Rules:

- Exactly one of `check` or `review` is required. A convention nobody enforces is not accepted by the format.
- A `must` needs a reason in the body.
- Unknown fields are errors.

Globs use doublestar syntax: `**/*.ts` for any depth, `src/**` for everything under `src/`. This is the same syntax Claude Code, Cursor and Copilot use for path-scoped rules, so `paths` means the same thing to opine and to the agent.

### Topics

The topic is a closed list, so that conventions are filed in the same places in every repository:

`layout` `naming` `boundaries` `design` `errors` `testing` `api` `data` `dependencies` `style` `docs` `workflow` `security` `performance` `observability`

### Checks

| `kind` | Fields | Fails when |
|---|---|---|
| `forbid` | `pattern`, optional `unless` | a line matches `pattern` and does not match `unless` |
| `require` | `pattern` | a file in `paths` does not contain `pattern` |
| `name` | `style`, optional `target: dir`, `pattern` when style is `regex` | a file or directory name is not in the style |
| `pair` | `needs` | a file in `paths` has no companion, e.g. `"{dir}/{name}.test{ext}"` |
| `exists` | `files` | none of the repository's files match an entry |
| `absent` | `files` | any file matches |
| `max-lines` | `max` | a file is longer |
| `run` | `cmd` | the command exits non-zero |

Notes:

- Patterns are RE2 regular expressions, matched line by line for `forbid`. RE2 has no lookahead; `unless` covers the common need for one.
- `name` styles are `kebab-case`, `snake_case`, `camelCase`, `PascalCase` and `regex`. The style applies to the name up to its first dot, ignoring leading and trailing underscores; dotfiles are skipped.
- `pair` placeholders: `{dir}` the file's directory, `{name}` its name without the last extension, `{stem}` the name up to the first dot, `{ext}` the last extension. `needs` may list alternatives.
- `run` wraps a tool the repository already has, such as a linter or an architecture test. **It is only allowed in a repository's own conventions.** A pack cannot run commands.
- `message` on any check replaces the rule as the text shown for a violation.
- Binary files and files over 1 MB are skipped by content checks.

### Exceptions

To allow one exception, comment on the line or the line above:

```ts
// opine-ignore boundaries/database-only-in-core: the migration tool needs the raw driver
import Database from "better-sqlite3";
```

For a whole file, within its first ten lines:

```ts
/* opine-ignore-file observability/no-console: this is the CLI entry point */
```

The reason is required. A suppression without one is itself a `must` violation.

## Decisions

A decision is a question every codebase answers. A pack declares it in `decisions.yaml`:

```yaml
errors/propagation:
  question: How does a function report failure?
```

and ships one convention per answer, each marked with `decides: errors/propagation` and an `option`. A repository answers in one line:

```yaml
decisions:
  errors/propagation: results
```

Only the chosen option's conventions are in force. `none` is always a valid answer and means "we considered this and deliberately have no rule." A decision with no answer is undecided: none of its options apply, and `opine decisions` lists it.

A `default` on a decision is what `opine init` picks for a new repository. It is written into `opine.yaml` explicitly; nothing is ever applied by default without appearing there.

## Structure

`.opine/structure.yaml` describes the folder layout as a tree. Keys ending in `/` are directories; other keys are attributes of the directory they sit in.

```yaml
tree:
  src/:
    closed: true
    files: kebab-case
    api/:     { role: interface, doc: "HTTP handlers. No business logic" }
    domain/:  { role: core, doc: "Business rules. No I/O" }
    infra/:   { role: adapter }
    features/:
      "{feature}/":
        role: feature
        isolated: true
        requires: [index.ts]
  docs/:

layers:
  - interface
  - feature | adapter
  - core

aliases:
  "@/": src/
```

| Attribute | Meaning |
|---|---|
| `doc` | What belongs here. Shown to agents and in CONVENTIONS.md. |
| `role` | A name for what the directory is, referred to by `layers`. |
| `closed: true` | No subdirectories other than the ones listed. |
| `files`, `dirs` | Naming style for files and directories below, until a deeper directory says otherwise. |
| `requires` | Paths every such directory must contain. |
| `"{name}/"` | A placeholder: stands for any directory at that level. One per level. |
| `isolated: true` | On a placeholder: its directories may not import each other. |

`layers` lists roles from highest to lowest. A file may import only from layers below its own. Within one layer, `a | b` means the two may not import each other, and `a + b` means they may. A role not named in `layers` is unconstrained.

`aliases` maps an import prefix to a path, for imports that are not relative.

How imports are resolved:

| Imports | Resolution |
|---|---|
| Relative paths (JavaScript, TypeScript, C and C++ includes, Ruby `require_relative`, Python relative imports) | Exact |
| Aliases | Exact |
| Go | Exact, using the module path in `go.mod` |
| Fully-qualified names (Java, Kotlin, Scala, Python, C#, PHP, Rust `crate::`) | A name is matched to a role directory when it begins with the trailing segments of that directory's path: `com.acme.infra.Db` resolves to `.../com/acme/infra` |

An import that resolves to no role directory is treated as external and ignored.

Structure findings are reported under `layout/structure`, `naming/structure` and `boundaries/layers`, at `level: must` unless the file sets `level: should`. They honour suppressions and the baseline like any other.

## Configuration

`.opine/opine.yaml`:

```yaml
version: 1

extends:
  - builtin:universal
  - path: ../house-style
  - git: https://github.com/acme/house-style
    ref: v1.4.0

decisions:
  errors/propagation: results

overrides:
  docs/readme: { level: off, reason: "This is a library fragment; the parent repo documents it" }
  security/no-secrets: { exclude: ["testdata/**"], reason: "Fixtures contain fake keys" }

ignore: ["vendor/**", "**/*.generated.*"]

emit:
  targets: [agents, claude, cursor, copilot, digest]
  budget: 60
```

### Packs and the cascade

`extends` lists layers from lowest to highest; the repository's own `.opine/` is always the top layer. When two layers define the same id, the higher one wins, and `opine explain <id>` shows the chain.

A pack is a directory with `pack.yaml`, `conventions/` and optionally `decisions.yaml`. Any repository that uses opine can also be extended directly: its `.opine/` is then the pack.

Git packs are fetched by `opine sync` into `.opine/cache/` and pinned in `.opine/opine.lock`. A pinned commit is used until `opine sync --update`. In CI, run `opine sync` before `opine check`.

Packs do not yet extend other packs.

### Overrides

An override changes `level`, `paths` or `exclude` for an inherited convention and must give a `reason`. `level: off` turns it off. The override and its reason appear in `opine explain`.

## The baseline

`opine check --update-baseline` records, per convention and file, how many violations exist. From then on a file may not exceed its count. Counts rather than line numbers are stored, so unrelated edits do not disturb the baseline. When a file goes over its count, all of its violations of that convention are reported, because a count cannot say which one is new.

## What `emit` writes

| File | Content |
|---|---|
| `AGENTS.md` | Between `<!-- opine:begin -->` and `<!-- opine:end -->`: one line per repository-wide convention, and where code goes. Everything outside the markers is yours and is never changed. |
| `CLAUDE.md` | Created with `@AGENTS.md` if absent. If present without that import, the import is added inside markers. |
| `.claude/rules/opine/*.md` | One file per path-scoped convention, with `paths:` frontmatter, so it loads only when matching files are touched. |
| `.cursor/rules/opine-*.mdc` | The same, for Cursor. Written if `.cursor/` exists. |
| `.github/instructions/opine-*.instructions.md` | The same, for Copilot. Written if Copilot instructions exist. |
| `CONVENTIONS.md` | The digest for people: decisions, structure, and one line per convention. |

`emit.targets` overrides which are written. `emit.budget` is the number of lines the AGENTS.md block may reach before `emit` warns.

## Exit codes

| Command | 0 | 1 | 2 |
|---|---|---|---|
| `check` | no `must` violations | `must` violations | bad configuration or usage |
| `check --hook` | nothing to report | configuration problem | violations in the edited file, on stderr |
| `emit --check` | up to date | out of date | bad configuration or usage |
| `lint` | valid | problems | usage |

`opine check --format json` and `--format sarif` write machine-readable results; SARIF can be uploaded to GitHub code scanning.
