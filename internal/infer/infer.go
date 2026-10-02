// Package infer reads what an existing repository already does, so that
// 'opine init' can start from the code rather than from a blank page.
//
// Everything here is counting, not judgment: a finding is only reported when
// the repository is nearly unanimous, and it carries the counts so a person
// can see at a glance whether it is true.
package infer

import (
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/vora-sahil/opinionated/internal/check"
)

const (
	minNamed     = 8    // files with an unambiguous style before naming is inferred
	namingShare  = 0.85 // share that must agree
	minTests     = 5
	testsShare   = 0.80
	maxExamples  = 3
	maxChildDirs = 12
)

var langByExt = map[string]string{
	".ts": "TypeScript", ".tsx": "TypeScript", ".js": "JavaScript", ".jsx": "JavaScript", ".mjs": "JavaScript",
	".py": "Python", ".kt": "Kotlin", ".kts": "Kotlin", ".java": "Java", ".go": "Go", ".rs": "Rust",
	".cs": "C#", ".rb": "Ruby", ".php": "PHP", ".swift": "Swift", ".c": "C", ".h": "C",
	".cpp": "C++", ".cc": "C++", ".hpp": "C++", ".scala": "Scala", ".dart": "Dart",
	".ex": "Elixir", ".exs": "Elixir", ".sh": "Shell",
}

var agentFiles = []string{
	"AGENTS.md", "CLAUDE.md", ".claude/rules", ".cursorrules", ".cursor/rules",
	".github/copilot-instructions.md", ".github/instructions", "GEMINI.md", ".windsurfrules", ".clinerules",
}

var linterFiles = []string{
	".editorconfig", "eslint.config.js", "eslint.config.mjs", "eslint.config.ts", ".eslintrc", ".eslintrc.js",
	".eslintrc.json", ".eslintrc.cjs", ".prettierrc", ".prettierrc.json", "prettier.config.js", "biome.json",
	"ruff.toml", ".ruff.toml", ".flake8", "mypy.ini", "detekt.yml", "config/detekt/detekt.yml", ".golangci.yml",
	".golangci.yaml", "rustfmt.toml", "clippy.toml", ".rubocop.yml", "checkstyle.xml", ".ls-lint.yml",
	".importlinter", ".dependency-cruiser.js", ".dependency-cruiser.cjs", "phpstan.neon", ".swiftlint.yml",
}

type Count struct {
	Name string
	N    int
}

type Naming struct {
	Language string
	Exts     []string
	Style    string
	Follows  int
	Violates int
	Examples []string // files that do not follow it
}

type Tests struct {
	Choice    string // colocated or separate
	Colocated int
	Separate  int
}

type Report struct {
	Languages  []Count
	AgentFiles []string
	Linters    []string
	Naming     []Naming
	Tests      *Tests
	SourceN    int
	Tree       map[string][]string // top-level directory -> its child directories
	TopDirs    []string
}

var conventional = regexp.MustCompile(`^(feat|fix|docs|style|refactor|perf|test|build|ci|chore|revert)(\([^)]+\))?!?: `)

const (
	minCommits   = 20
	commitsShare = 0.85
)

// Commits reports whether recent history follows Conventional Commits, as
// (matching, total). Only that style is detected, because only it can be
// recognised without guessing.
func Commits(root string) (n, of int, ok bool) {
	out, err := exec.Command("git", "-C", root, "log", "-n", "200", "--no-merges", "--format=%s").Output()
	if err != nil {
		return 0, 0, false
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		of++
		if conventional.MatchString(line) {
			n++
		}
	}
	return n, of, of >= minCommits && float64(n)/float64(of) >= commitsShare
}

var (
	testName = regexp.MustCompile(`(\.(test|spec)\.[a-z]+$)|(_test\.(go|py|rb)$)|(^test_.*\.py$)|(Tests?\.(kt|java|cs|swift)$)`)
	testDirs = map[string]bool{"test": true, "tests": true, "spec": true, "specs": true}
)

func Analyze(root string, files []string) *Report {
	r := &Report{Tree: map[string][]string{}}
	has := func(p string) bool {
		_, err := os.Stat(filepath.Join(root, filepath.FromSlash(p)))
		return err == nil
	}
	for _, f := range agentFiles {
		if has(f) {
			r.AgentFiles = append(r.AgentFiles, f)
		}
	}
	linterSet := map[string]bool{}
	for _, f := range linterFiles {
		linterSet[f] = true
	}
	seenLinter := map[string]bool{}
	for _, f := range files {
		if base := path.Base(f); (linterSet[base] || linterSet[f]) && !seenLinter[base] {
			seenLinter[base] = true
			r.Linters = append(r.Linters, base)
		}
	}

	langN := map[string]int{}
	type tally struct {
		styles map[string][]string
		exts   map[string]bool
	}
	byLang := map[string]*tally{}
	tests := &Tests{}
	top := map[string]map[string]bool{}

	for _, f := range files {
		segs := strings.Split(f, "/")
		if len(segs) > 1 && !strings.HasPrefix(segs[0], ".") {
			if top[segs[0]] == nil {
				top[segs[0]] = map[string]bool{}
			}
			if len(segs) > 2 && !strings.HasPrefix(segs[1], ".") {
				top[segs[0]][segs[1]] = true
			}
		}
		base := path.Base(f)
		ext := path.Ext(base)
		lang, ok := langByExt[ext]
		if !ok {
			continue
		}
		langN[lang]++
		r.SourceN++

		if testName.MatchString(base) {
			separate := strings.Contains(f, "src/test/")
			for _, s := range segs[:len(segs)-1] {
				separate = separate || testDirs[s]
			}
			if separate {
				tests.Separate++
			} else {
				tests.Colocated++
			}
		}

		if style := check.StyleOf(base); style != "" {
			t := byLang[lang]
			if t == nil {
				t = &tally{styles: map[string][]string{}, exts: map[string]bool{}}
				byLang[lang] = t
			}
			t.styles[style] = append(t.styles[style], f)
			t.exts[ext] = true
		}
	}

	for l, n := range langN {
		r.Languages = append(r.Languages, Count{l, n})
	}
	sort.Slice(r.Languages, func(i, j int) bool {
		a, b := r.Languages[i], r.Languages[j]
		return a.N > b.N || (a.N == b.N && a.Name < b.Name)
	})

	for lang, t := range byLang {
		total, best, bestN := 0, "", 0
		for style, fs := range t.styles {
			total += len(fs)
			if len(fs) > bestN || (len(fs) == bestN && style < best) {
				best, bestN = style, len(fs)
			}
		}
		if total < minNamed || float64(bestN)/float64(total) < namingShare {
			continue
		}
		n := Naming{Language: lang, Style: best, Follows: bestN, Violates: total - bestN}
		for e := range t.exts {
			n.Exts = append(n.Exts, e)
		}
		sort.Strings(n.Exts)
		for style, fs := range t.styles {
			if style != best {
				n.Examples = append(n.Examples, fs...)
			}
		}
		sort.Strings(n.Examples)
		if len(n.Examples) > maxExamples {
			n.Examples = n.Examples[:maxExamples]
		}
		r.Naming = append(r.Naming, n)
	}
	sort.Slice(r.Naming, func(i, j int) bool { return r.Naming[i].Language < r.Naming[j].Language })

	if total := tests.Colocated + tests.Separate; total >= minTests {
		switch {
		case float64(tests.Colocated)/float64(total) >= testsShare:
			tests.Choice = "colocated"
		case float64(tests.Separate)/float64(total) >= testsShare:
			tests.Choice = "separate"
		}
		if tests.Choice != "" {
			r.Tests = tests
		}
	}

	for d, kids := range top {
		r.TopDirs = append(r.TopDirs, d)
		if len(kids) > maxChildDirs {
			continue
		}
		for k := range kids {
			r.Tree[d] = append(r.Tree[d], k)
		}
		sort.Strings(r.Tree[d])
	}
	sort.Strings(r.TopDirs)
	return r
}
