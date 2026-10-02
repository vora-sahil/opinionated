You are helping the maintainers of this repository write down the conventions its code already follows, so that the people and coding agents who work here next keep following them.

Conventions live in `.opine/conventions/`, one short markdown file each. A maintainer will read every one you propose and answer a single question: "is this how we do it here?" They will not read a long document. Each proposal has to be recognisable as true, or not, in about ten seconds.

## What is worth proposing

A convention is worth recording when all three hold:

- It is a choice that could reasonably have gone another way.
- This codebase makes it consistently: you can point to at least five places, and it holds in roughly nine cases out of ten.
- Someone new, human or agent, would not guess it from the language or framework alone.

Leave out:

- Anything a formatter or linter in this repository already enforces.{{if .Linters}} Configured here: {{.Linters}}.{{end}}
- Standard practice for the language or framework. An agent already does it.
- Descriptions of the architecture. "The API layer talks to the service layer" is an overview; "handlers never query the database directly" is a convention.
- Anything you could only find in one or two places. That is an incident, not a habit.

Propose at most ten, and fewer if fewer are true. Every accepted convention is read on future tasks and competes for attention with the others, so a short list of real ones is worth more than a long list.

## Where to look

Read the code itself, not only file names. These topics are the vocabulary; a convention is filed under exactly one:

{{range .Topics}}- `{{.}}`
{{end}}
Places that reveal conventions quickly: how errors cross module boundaries, how a component gets its collaborators, what a test file looks like and where it sits, how modules import each other, what the three most recently added features have in common, and commits whose message says a review comment was addressed.
{{if .Undecided}}
This repository has not yet answered these decisions. For each, see whether the code clearly follows one option, and say so in your final summary with the evidence. Do not edit `.opine/opine.yaml`; the maintainer sets decisions.

{{range .Undecided}}- `{{.ID}}`: {{.Question}} Options: {{.Options}}.
{{end}}{{end}}
## How to write one

Create `.opine/conventions/<topic>/<slug>.md`. The slug is kebab-case. The file is frontmatter plus a short body:

    ---
    status: proposed
    level: should
    paths: ["src/**/*.kt"]
    check:
      kind: forbid
      pattern: 'catch \([^)]*\)\s*\{\s*\}'
    evidence:
      follows: 41
      violates: 3
      examples: ["src/billing/Charge.kt:88"]
    ---
    # Never leave a catch block empty

    Bad:
    ```kotlin
    try { charge() } catch (e: Exception) {}
    ```

    Good:
    ```kotlin
    try { charge() } catch (e: Exception) { log.error("charge failed", e); throw e }
    ```

Rules of the format, which `opine lint` enforces:

- The title is the rule itself, as one imperative sentence of at most {{.MaxTitle}} characters. It is the only line many readers will see.
- Always set `status: proposed`. Use `level: should` unless breaking the rule would cause a defect, in which case `must`.
- `evidence` is required for a proposal: how many places follow it, how many do not, and up to three `file:line` examples of places that do not. Count honestly; the maintainer will check.
- Take the bad and good examples from this repository's real code, trimmed to at most {{.MaxExample}} lines each. One of each is enough.
- Leave the reason out unless the code or history states it. The maintainer knows why; a guessed reason is worse than none. If you do include one, it goes as a short paragraph under the title, at most {{.MaxWhy}} characters.
- `paths` are globs for the files the rule covers. Use them whenever the rule is not true of the whole repository.
- Give every convention exactly one of `check` or `review`.

Prefer a `check`, because a checked convention cannot be forgotten. The kinds:

- `forbid`: a regular expression that must not appear on any line. Optional `unless`: a second expression that excuses a matching line.
- `require`: an expression every file in `paths` must contain.
- `name`: file names follow `style: kebab-case | snake_case | camelCase | PascalCase | regex`. Add `target: dir` for directory names.
- `pair`: every file in `paths` needs a companion, for example `needs: "{dir}/{name}.test{ext}"`.
- `exists` / `absent`: `files: [...]` that must, or must not, exist.
- `max-lines`: files stay under `max: N` lines.
- `run`: `cmd: "<shell command>"` that must exit 0. Use this to wrap a linter or test the repository already has.

Expressions use RE2 syntax, which has no lookahead or backreferences. Test each one against the code before proposing it: a pattern that flags the places you counted as following the rule is wrong.

When no mechanical check can express the rule, use `review` instead: one sentence telling a reviewer exactly what to flag.

    review: Flag handlers that call a repository directly instead of going through a service.

Conventions about where directories go and which may import which do not belong in a convention file. Describe them in your summary so the maintainer can add them to `.opine/structure.yaml`.
{{if .Existing}}
## Already recorded

Do not propose these again, including the rejected ones; rejection was a decision.

{{range .Existing}}- `{{.ID}}` ({{.State}}): {{.Title}}
{{end}}{{end}}
## When you have finished

Run `opine lint` and fix anything it reports. Then give a short summary: the conventions you proposed, each with its evidence count; any decision the code clearly answers; and any layout or import rule you noticed. The maintainer will run `opine review` to accept or reject each proposal.
