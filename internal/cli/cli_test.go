package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vora-sahil/opinionated/internal/testrepo"
)

type result struct {
	code     int
	out, err string
}

func opine(t *testing.T, dir, stdin string, args ...string) result {
	t.Helper()
	var out, errb bytes.Buffer
	code := Run(args, dir, strings.NewReader(stdin), &out, &errb)
	return result{code, out.String(), errb.String()}
}

func (r result) want(t *testing.T, code int, substrings ...string) {
	t.Helper()
	if r.code != code {
		t.Fatalf("exit %d, want %d\nstdout:\n%s\nstderr:\n%s", r.code, code, r.out, r.err)
	}
	for _, s := range substrings {
		if !strings.Contains(r.out+r.err, s) {
			t.Errorf("missing %q in:\n%s%s", s, r.out, r.err)
		}
	}
}

func read(t *testing.T, root, p string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, p))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func existingRepo(t *testing.T) string {
	files := map[string]string{
		"README.md": "# shop\n",
		"CLAUDE.md": "# Notes for Claude\n\nUse pnpm, not npm.\n",
	}
	for _, n := range []string{"order-list", "order-item", "cart-view", "cart-total", "user-menu", "user-card", "price-tag", "tax-rule", "ship-cost"} {
		files["src/"+n+".ts"] = "export const x = 1\n"
		files["src/"+n+".test.ts"] = "test('x', () => {})\n"
	}
	// opine-ignore workflow/track-todos: a fixture; the untracked TODO is what the test needs
	files["src/LegacyThing.ts"] = "// TODO tidy\nexport const y = 2\n"
	return testrepo.New(t, files)
}

func TestLifecycleOnAnExistingRepo(t *testing.T) {
	root := existingRepo(t)

	opine(t, root, "", "init").want(t, 0,
		"TypeScript (19)", "CLAUDE.md",
		"layout/tests = colocated  (inferred: 9 of 9 test files sit beside the code)",
		"naming/typescript-files   Name TypeScript files in kebab-case (18 of 19 files already do)")
	opine(t, root, "", "init").want(t, 2, "already exists")
	opine(t, root, "", "lint").want(t, 0)

	// Nothing proposed is enforced until a person says yes.
	opine(t, root, "", "check").want(t, 0, "No violations.")
	opine(t, root, "y\n", "review").want(t, 0, "18 of 19 places already do this", "not yet: src/LegacyThing.ts", "Accepted 1")
	if rec := read(t, root, ".opine/conventions/naming/typescript-files.md"); strings.Contains(rec, "proposed") || strings.Contains(rec, "evidence") {
		t.Errorf("accepting should drop status and evidence:\n%s", rec)
	}
	opine(t, root, "", "check").want(t, 0, "src/LegacyThing.ts", "naming/typescript-files", "1 violation: 0 must, 1 should")

	// The baseline grandfathers what exists and holds the line after it.
	opine(t, root, "", "check", "--update-baseline").want(t, 0, "1 violation")
	opine(t, root, "", "check").want(t, 0, "No violations.", "1 more covered by the baseline")
	testrepo.Write(t, root, map[string]string{"src/AnotherOne.ts": "export {}\n", "src/AnotherOne.test.ts": ""})
	opine(t, root, "", "check").want(t, 0, "src/AnotherOne.ts")
	opine(t, root, "", "check", "--no-baseline", "--format", "json").want(t, 0, `"file": "src/LegacyThing.ts"`)

	// Emit extends what a person already wrote and never rewrites it.
	opine(t, root, "", "emit").want(t, 0, "AGENTS.md", "CONVENTIONS.md", ".claude/rules/opine/naming-typescript-files.md")
	claude := read(t, root, "CLAUDE.md")
	if !strings.HasPrefix(claude, "# Notes for Claude\n\nUse pnpm, not npm.\n") || !strings.Contains(claude, "@AGENTS.md") {
		t.Errorf("CLAUDE.md should keep its content and gain the import:\n%s", claude)
	}
	opine(t, root, "", "emit", "--check").want(t, 0, "up to date")
	opine(t, root, "", "emit").want(t, 0, "Nothing to do")
	testrepo.Write(t, root, map[string]string{"AGENTS.md": "# My notes\n\n" + read(t, root, "AGENTS.md") + "\nMore of mine.\n"})
	opine(t, root, "", "emit").want(t, 0, "Nothing to do")
	if a := read(t, root, "AGENTS.md"); !strings.HasPrefix(a, "# My notes\n") || !strings.HasSuffix(a, "More of mine.\n") {
		t.Errorf("content outside the markers changed:\n%s", a)
	}

	opine(t, root, "", "explain", "naming/typescript-files").want(t, 0, "in force", "file names in kebab-case", "opine-ignore naming/typescript-files")
	opine(t, root, "", "explain", "nope/nothing").want(t, 2, "no convention")
	opine(t, root, "", "decisions").want(t, 0, "= colocated", "= undecided")
	opine(t, root, "", "prompt", "mine").want(t, 0, "status: proposed", "`naming/typescript-files` (in force)", "errors/propagation")
	opine(t, root, "", "prompt", "review").want(t, 0, "errors/no-silent-failure", "What to flag:")
}

func TestNewRepoGetsTheBaseline(t *testing.T) {
	root := testrepo.New(t, map[string]string{"README.md": "# new\n"})
	opine(t, root, "", "init").want(t, 0, "baseline for a new repository", "design/wiring = explicit")
	opine(t, root, "", "check").want(t, 0, "No violations.")
	opine(t, root, "", "emit").want(t, 0)
	agents := read(t, root, "AGENTS.md")
	for _, s := range []string{"Never commit a secret", "Pass a component its dependencies", "Group code by feature"} {
		if !strings.Contains(agents, s) {
			t.Errorf("AGENTS.md missing %q:\n%s", s, agents)
		}
	}
	if got := read(t, root, "CLAUDE.md"); got != "@AGENTS.md\n" {
		t.Errorf("CLAUDE.md = %q", got)
	}
}

func TestHookReportsOnlyTheEditedFile(t *testing.T) {
	root := testrepo.New(t, map[string]string{
		".opine/opine.yaml": "version: 1\n",
		".opine/conventions/observability/no-console.md": "---\nlevel: must\npaths: ['**/*.js']\ncheck: {kind: forbid, pattern: 'console\\.log'}\n---\n# No console.log\n\nUse the logger.\n",
		"a.js": "console.log(1)\n",
		"b.js": "console.log(2)\n",
	})
	in := `{"tool_name":"Edit","tool_input":{"file_path":"` + filepath.Join(root, "b.js") + `"}}`
	r := opine(t, root, in, "check", "--hook")
	r.want(t, 2, "b.js", "observability/no-console")
	if strings.Contains(r.err, "a.js") || r.out != "" {
		t.Errorf("hook must report only the edited file, on stderr:\nout=%s\nerr=%s", r.out, r.err)
	}
	clean := `{"tool_input":{"file_path":"` + filepath.Join(root, ".opine/opine.yaml") + `"}}`
	opine(t, root, clean, "check", "--hook").want(t, 0)
	opine(t, root, `{"tool_input":{"file_path":"/elsewhere/x.js"}}`, "check", "--hook").want(t, 0)
	opine(t, root, "not json", "check", "--hook").want(t, 0)
}

func TestBrokenConfigurationStopsEverything(t *testing.T) {
	root := testrepo.New(t, map[string]string{
		".opine/opine.yaml":                     "version: 1\ndecisions:\n  errors/propagation: vibes\noverrides:\n  docs/readme: {level: off}\n",
		".opine/conventions/errors/too-long.md": "---\nreview: look\n---\n# " + strings.Repeat("word ", 30) + "\n",
	})
	r := opine(t, root, "", "check")
	r.want(t, 2, "title is", "no such decision", "no convention with this id", "Nothing was checked")
	opine(t, root, "", "lint").want(t, 1)
}

func TestReviewRejectIsRemembered(t *testing.T) {
	root := testrepo.New(t, map[string]string{
		".opine/opine.yaml":                    "version: 1\n",
		".opine/conventions/data/plain-sql.md": "---\nstatus: proposed\nlevel: must\nreview: Flag ORMs.\n---\n# Write plain SQL\n",
	})
	opine(t, root, "", "review", "--accept", "data/plain-sql").want(t, 2, "needs a reason")
	opine(t, root, "", "review", "--reject", "data/plain-sql", "--reason", "we use an ORM").want(t, 0)
	opine(t, root, "", "list", "--all").want(t, 0, "rejected: we use an ORM")
	opine(t, root, "", "review").want(t, 0, "Nothing is waiting")
}
