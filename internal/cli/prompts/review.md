Review the change below against this repository's conventions that no machine can check. The mechanical ones are already enforced by `opine check`; do not repeat its work.

Report only violations this change introduces. Code the change did not touch is out of scope, even if it breaks a convention. If the change breaks none, say so in one line and stop.

For each violation give the file and line, the convention id, what is wrong, and the smallest fix, in this form:

    path/to/file.ext:42  errors/no-silent-failure
    The catch block discards the error. Log and rethrow it, or return a failure.

A convention with `must` is a defect to fix before merging. One with `should` is a strong default: if the change has a good reason to deviate, say what the reason appears to be rather than flagging it.

## Conventions to review against

{{range .Conventions}}### {{.Title}}
`{{.ID}}` · {{.Level}}{{if .Paths}} · applies to {{.Paths}}{{end}}
{{if .Why}}
{{.Why}}
{{end}}
What to flag: {{.Review}}

{{end}}{{if not .Conventions}}There are no review-enforced conventions in force, so there is nothing to review.
{{end}}## The change

Review the diff you are given after this prompt. If none is given, review `git diff` against the main branch.
