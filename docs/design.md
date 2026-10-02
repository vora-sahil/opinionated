# Design

Why opine is shaped the way it is, what each choice was learned from, and what was considered and turned down.

## The problem

A codebase's conventions have always lived in two places: the code, and the heads of the people who wrote it. New people absorbed them through review. That mechanism is weakening. An agent writes plausible, working code that follows the conventions it can see in the files it happens to read and the averages of everything it was trained on. Nobody reviews it line by line the way a junior engineer's first pull requests were reviewed. The opinions are not rejected; they are just no longer transmitted.

The existing answers each cover one part:

| Kind | Examples | What is missing |
|---|---|---|
| A context file | AGENTS.md, CLAUDE.md, Cursor rules | Prose with no enforcement and no reason attached; grows until nobody reads it |
| Format sync | rulesync, agent_sync | Moves the prose between tools; says nothing about what to write |
| Standards as documents | Agent OS | Prose again; tied to one workflow |
| Generators | Caliber, Sourcebook, Archprint | One shot, no human gate, no reason captured, no lifecycle |
| Linters for context files | ctxlint, agentlint | Check that the document is well formed, not that the code still follows it |
| Linters | eslint, ruff, detekt, clippy | Enforce well, per language, but hold no reasons and cannot express judgment |

The gap is the thing in the middle: a convention as a first-class record that carries its reason and its enforcement together, that a person can approve in seconds, and that travels between repositories.

## Four questions the design had to answer

### 1. How does a person read one and say "yes, that is how we do it"?

Nobody reads a long generated document, and there is evidence they should not have to:

- A study of AGENTS.md files across several agents and models found that context files did not generally improve task success and raised cost by over 20%. Instructions in them were followed, but repository overviews, the most commonly recommended content, did not help. The authors conclude context files are useful for stating non-standard practices and little else. ([Evaluating AGENTS.md](https://arxiv.org/abs/2602.11988))
- Claude Code's own guidance is to keep instruction files under 200 lines, be specific enough to verify, and move anything that applies to part of the codebase into path-scoped rules. ([Claude Code memory](https://code.claude.com/docs/en/memory))
- Review research has long found that attention is finite: defect detection falls off sharply past a few hundred lines. ([SmartBear and Cisco](https://smartbear.com/learn/code-review/best-practices-for-peer-code-review/))
- Developers learn conventions from examples faster than from prose, which is why every mature linter documents a rule as what it does, why, and a bad and good example. ([Clippy](https://doc.rust-lang.org/clippy/lints.html), ESLint)
- Naturalize, which mined naming conventions statistically, got 14 of 18 generated patches accepted by open-source maintainers: a convention presented as a concrete instance with evidence is easy to say yes to. ([Allamanis et al.](https://arxiv.org/abs/1402.4182))

So the unit is a **card**, not a document:

- The title is the rule, in one sentence of at most 100 characters.
- One short paragraph of reason.
- One bad and one good example, from the repository's own code.
- For a proposal, **evidence**: how many places already follow it and how many do not. "41 of 44" is recognisable as true or false at a glance.

The limits are enforced by `opine lint`, not suggested. `opine review` shows one card at a time and asks one question. `CONVENTIONS.md` is one line per convention. The long form, where it exists, is an ADR linked with `see:`.

The same evidence shapes what agents are given. Only repository-wide conventions go into AGENTS.md, one line each, with no overview. Anything scoped to paths becomes a rule file that loads only when matching files are touched. A convention that a check fully covers can set `emit: false` and cost the agent nothing.

### 2. Does it need enums?

Yes, wherever a machine branches or two repositories need to be comparable, and nowhere else.

| Field | Values | Borrowed from |
|---|---|---|
| `level` | `must`, `should`, `off` | RFC 2119 for the words people already use; ESLint's off, warn, error for the behaviour |
| `status` | `proposed`, `accepted`, `rejected`, `deprecated` | [MADR](https://adr.github.io/madr/) decision records |
| topic | 15 fixed topics | Clippy's lint groups, the sections of the Google style guides |
| `check.kind` | 8 kinds | See below |

The topic is the directory a convention sits in, so it is never typed twice and cannot disagree with itself. A fixed list means an engineer who moves between companies finds error handling under `errors/` and module rules under `boundaries/` in both.

The rule, the reason, and tags stay free text. Rejected conventions are kept with their reason rather than deleted, as MADR keeps rejected decisions, so the same proposal does not come back and the next person can see it was considered.

Two levels are enough. A third, advisory level was considered and dropped: a convention nobody is expected to follow is not a convention.

### 3. Does it need a catalog of patterns?

Not of patterns. Of **decisions**.

A catalog of design patterns with detectors would be enormous, would duplicate decades of existing literature, and pattern detection is unreliable. But there is a much smaller and more useful list: the questions every codebase answers whether or not anyone wrote the answer down. How does a function report failure? Where do tests live? How does a component get its dependencies?

The precedent is EditorConfig. It succeeded not by having opinions but by standardising the questions (`indent_style`, `end_of_line`) and letting each project answer. MADR's "considered options" and the Google style guides' pros, cons and decision format are the same idea applied to design.

So a pack declares decisions, each with named options, and each option is an ordinary convention. A repository answers in one line:

```yaml
decisions:
  errors/propagation: results
  layout/tests: colocated
```

That block is the densest representation of a codebase's opinions there is, and it is the part that makes opinions portable: two repositories can be compared answer by answer. `none` is always a valid answer, so "we deliberately have no rule" is recorded rather than indistinguishable from "nobody thought about it." Nothing applies by default without appearing in the file.

Conventions that are not an answer to a shared question are still first-class; most local opinions are exactly that.

### 4. Is there a standard way to represent folder structure?

There was not one; there were three partial ones, each solving a different third:

| Concern | Prior art |
|---|---|
| Shape: what may exist where, and how it is named | [ls-lint](https://ls-lint.org), eslint-plugin-project-structure |
| Roles: what each folder is for | Nx tags, [Feature-Sliced Design](https://feature-sliced.design) layers and slices |
| Direction: which may import which | [import-linter](https://import-linter.readthedocs.io) layers, dependency-cruiser, ArchUnit, Konsist |

`structure.yaml` puts the three in one file, written as the annotated tree people already draw in READMEs: directories are keys ending in `/`, and `doc`, `role`, `closed`, `files` and `requires` annotate them. `layers` is import-linter's contract: roles from high to low, `|` for independent siblings. A placeholder such as `{feature}/` with `isolated: true` is Feature-Sliced Design's rule that slices do not import each other.

One departure from import-linter: it writes siblings that may import each other as `a : b`, which YAML parses as a mapping. opine uses `a + b`.

Shape and naming are pure filesystem checks and work everywhere. Direction needs imports. opine resolves the cases that can be resolved exactly (relative paths, aliases, Go module paths) and matches fully-qualified names by package path. It does not build a type-aware dependency graph; a repository that needs one wraps ArchUnit, Konsist or dependency-cruiser with a `run` check.

Agents are given the `doc` lines and the layer order as instructions ("handlers go here; imports only point down"), not the whole tree as an overview, for the reason in question 1.

## Other choices

**Every convention is enforced or it is not accepted.** A record must have a `check` or a `review` instruction. This is MADR's "confirmation" section made mandatory: if nobody can say how a violation would be noticed, the convention will not survive.

**Checks delegate; they do not parse.** The built-in kinds are deliberately shallow: patterns, names, file existence, pairs, length. Depth in a given language comes from that language's own tools, wrapped with `run` today and through native adapters later. Reimplementing eslint, ruff and detekt would be a worse version of each.

**A pack is data.** A pack cannot run commands. Extending a pack means running someone else's content in your CI; if that content could execute, a pack would be a supply-chain attack waiting for an author. `run` is allowed only in a repository's own conventions. Git packs are pinned to a commit in a lock file.

**A baseline, as counts.** Adopting a convention on existing code must not require fixing everything first, the problem ESLint's bulk suppressions and detekt's baseline solve. opine stores counts per convention and file rather than line numbers, so the baseline does not churn when code moves.

**Suppressions need a reason.** Following Clippy's `#[expect(lint, reason = "...")]`. An exception is a small decision and is recorded as one.

**Overrides need a reason.** A repository may deviate from an inherited convention, but the deviation and why are written down and shown by `opine explain`. That is the difference between an override and drift.

**opine makes no model calls.** Extraction and review need judgment, and the user already has an agent. `opine prompt mine` and `opine prompt review` print instructions for it; opine supplies the format, the validation and the human gate. No API key, and no tie to one vendor.

**Inference only counts.** `opine init` proposes a naming convention or a test location only when the repository is nearly unanimous, and shows the counts. It never guesses a reason. A wrong inferred convention costs more trust than a missing one.

**A single static binary.** A Kotlin or Python repository should not need Node to hold opinions.

**AGENTS.md is the target, not a competitor.** It is the open standard and most tools read it. opine writes inside its own markers and never touches what a person wrote outside them.

## Known limits

- Many real conventions are not mechanically checkable. For those, enforcement is a reviewer following an instruction, which is not deterministic.
- Fully-qualified import matching is by package path, so a third-party package whose name begins exactly like a local role directory's trailing path would be misread as local.
- The universal pack is small on purpose. Deep language packs do not exist yet.
- Whether a given set of conventions makes agents do better work in a given repository is an empirical question. The study above is a warning that more context is not automatically better; measure before assuming.
