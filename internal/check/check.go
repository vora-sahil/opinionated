// Package check runs the mechanical checks attached to conventions and
// applies suppressions and the baseline to what they find.
package check

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/vora-sahil/opinionated/internal/fsx"
	"github.com/vora-sahil/opinionated/internal/pack"
	"github.com/vora-sahil/opinionated/internal/record"
)

type Violation struct {
	ID      string       `json:"id"`
	Level   record.Level `json:"level"`
	File    string       `json:"file,omitempty"`
	Line    int          `json:"line,omitempty"`
	Message string       `json:"message"`
}

const maxFileSize = 1 << 20

// Source reads repository files once and hands out their lines.
type Source struct {
	Root  string
	cache map[string][]string
}

func NewSource(root string) *Source { return &Source{Root: root, cache: map[string][]string{}} }

// Lines returns the file's lines, or nil for binary, oversized or unreadable
// files, which no content check applies to.
func (s *Source) Lines(file string) []string {
	if l, ok := s.cache[file]; ok {
		return l
	}
	var lines []string
	data, err := os.ReadFile(filepath.Join(s.Root, filepath.FromSlash(file)))
	if err == nil && len(data) <= maxFileSize && !bytes.Contains(data[:min(len(data), 8000)], []byte{0}) {
		lines = strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
		if n := len(lines); n > 0 && lines[n-1] == "" {
			lines = lines[:n-1]
		}
	}
	s.cache[file] = lines
	return lines
}

// Options narrows a run. Only, when set, limits per-file checks to those
// files and skips repository-wide checks; it is what an editor hook uses.
type Options struct {
	Only    []string
	SkipRun bool
}

// Files returns the repository's files minus the configured ignores.
func Files(res *pack.Resolved) ([]string, error) {
	all, err := fsx.List(res.Root)
	if err != nil {
		return nil, err
	}
	out := all[:0]
	for _, f := range all {
		if !fsx.MatchAny(res.Config.Ignore, f) {
			out = append(out, f)
		}
	}
	return out, nil
}

// Run executes every active mechanical convention against files.
func Run(res *pack.Resolved, files []string, src *Source, opt Options) []Violation {
	only := map[string]bool{}
	for _, f := range opt.Only {
		only[f] = true
	}
	partial := len(opt.Only) > 0
	fileSet := map[string]bool{}
	for _, f := range files {
		fileSet[f] = true
	}

	var out []Violation
	for _, rec := range res.Active() {
		c := rec.Check
		if c == nil {
			continue
		}
		msg := c.Message
		if msg == "" {
			msg = rec.Title
		}
		add := func(file string, line int, m string) {
			out = append(out, Violation{ID: rec.ID, Level: rec.Level, File: file, Line: line, Message: m})
		}
		var scoped []string
		for _, f := range files {
			if partial && !only[f] {
				continue
			}
			if fsx.InScope(rec.Paths, rec.Exclude, f) {
				scoped = append(scoped, f)
			}
		}

		switch c.Kind {
		case "forbid":
			re, err := regexp.Compile(c.Pattern)
			if err != nil {
				continue
			}
			var unless *regexp.Regexp
			if c.Unless != "" {
				unless, _ = regexp.Compile(c.Unless)
			}
			for _, f := range scoped {
				for i, line := range src.Lines(f) {
					if re.MatchString(line) && (unless == nil || !unless.MatchString(line)) {
						add(f, i+1, msg)
					}
				}
			}
		case "require":
			re, err := regexp.Compile(c.Pattern)
			if err != nil {
				continue
			}
			for _, f := range scoped {
				lines := src.Lines(f)
				if lines != nil && !re.MatchString(strings.Join(lines, "\n")) {
					add(f, 0, msg)
				}
			}
		case "max-lines":
			for _, f := range scoped {
				if n := len(src.Lines(f)); n > c.Max {
					add(f, 0, fmt.Sprintf("%s (%d lines, limit %d)", msg, n, c.Max))
				}
			}
		case "name":
			re := nameRegexp(c)
			if re == nil {
				continue
			}
			if c.Target == "dir" {
				if partial {
					continue
				}
				for _, d := range fsx.Dirs(files) {
					if fsx.InScope(rec.Paths, rec.Exclude, d) && !nameOK(re, path.Base(d)) {
						add(d, 0, fmt.Sprintf("%s (directory %q is not %s)", msg, path.Base(d), c.Style))
					}
				}
				continue
			}
			for _, f := range scoped {
				if !nameOK(re, path.Base(f)) {
					add(f, 0, fmt.Sprintf("%s (%q is not %s)", msg, path.Base(f), c.Style))
				}
			}
		case "pair":
			for _, f := range scoped {
				found := false
				var want []string
				for _, tmpl := range c.Needs {
					p := expand(tmpl, f)
					want = append(want, p)
					if fileSet[p] {
						found = true
						break
					}
				}
				if !found {
					add(f, 0, fmt.Sprintf("%s (expected %s)", msg, strings.Join(want, " or ")))
				}
			}
		case "exists":
			if partial {
				continue
			}
			dirs := fsx.Dirs(files)
			for _, pat := range c.Files {
				if !matchesAny(pat, files) && !matchesAny(strings.TrimSuffix(pat, "/"), dirs) {
					add(pat, 0, fmt.Sprintf("%s (%s does not exist)", msg, pat))
				}
			}
		case "absent":
			for _, f := range files {
				if partial && !only[f] {
					continue
				}
				if fsx.MatchAny(c.Files, f) && !fsx.MatchAny(rec.Exclude, f) {
					add(f, 0, msg)
				}
			}
		case "run":
			if partial || opt.SkipRun {
				continue
			}
			cmd := exec.Command("sh", "-c", c.Cmd)
			cmd.Dir = res.Root
			if outp, err := cmd.CombinedOutput(); err != nil {
				add("", 0, fmt.Sprintf("%s\n    $ %s\n%s", msg, c.Cmd, indent(tail(string(outp), 20))))
			}
		}
	}
	Sort(out)
	return out
}

func Sort(v []Violation) {
	sort.SliceStable(v, func(i, j int) bool {
		a, b := v[i], v[j]
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.ID < b.ID
	})
}

func matchesAny(pat string, paths []string) bool {
	for _, p := range paths {
		if fsx.MatchAny([]string{pat}, p) {
			return true
		}
	}
	return false
}

var styles = map[string]*regexp.Regexp{
	"kebab-case": regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`),
	"snake_case": regexp.MustCompile(`^[a-z0-9]+(_[a-z0-9]+)*$`),
	"camelCase":  regexp.MustCompile(`^[a-z][a-zA-Z0-9]*$`),
	"PascalCase": regexp.MustCompile(`^[A-Z][a-zA-Z0-9]*$`),
}

// StyleOf names the case style a file or directory name follows, or "".
// A single lowercase word satisfies kebab, snake and camel alike, so it
// reports "" as well: it is evidence for none of them.
func StyleOf(base string) string {
	stem := Stem(base)
	if stem == "" {
		return ""
	}
	var hit []string
	for _, s := range []string{"kebab-case", "snake_case", "camelCase", "PascalCase"} {
		if styles[s].MatchString(stem) {
			hit = append(hit, s)
		}
	}
	if len(hit) == 1 {
		return hit[0]
	}
	return ""
}

// Stem is the part of a name that a case style applies to: everything before
// the first dot, without leading or trailing underscores. Dotfiles have none.
func Stem(base string) string {
	if strings.HasPrefix(base, ".") {
		return ""
	}
	if i := strings.Index(base, "."); i >= 0 {
		base = base[:i]
	}
	return strings.Trim(base, "_")
}

// StyleMatch reports whether stem follows the named case style.
func StyleMatch(style, stem string) bool {
	re, ok := styles[style]
	return !ok || stem == "" || re.MatchString(stem)
}

func nameRegexp(c *record.Check) *regexp.Regexp {
	if c.Style == "regex" {
		re, err := regexp.Compile(c.Pattern)
		if err != nil {
			return nil
		}
		return re
	}
	return styles[c.Style]
}

func nameOK(re *regexp.Regexp, base string) bool {
	stem := Stem(base)
	return stem == "" || re.MatchString(stem)
}

// expand fills a pair template for file f. {dir} is its directory, {name} its
// base name without the last extension, {stem} the name before the first dot,
// {ext} the last extension including the dot.
func expand(tmpl, f string) string {
	base := path.Base(f)
	ext := path.Ext(base)
	stem := base
	if i := strings.Index(base, "."); i > 0 {
		stem = base[:i]
	}
	r := strings.NewReplacer(
		"{dir}", path.Dir(f),
		"{name}", strings.TrimSuffix(base, ext),
		"{stem}", stem,
		"{ext}", ext,
	)
	return path.Clean(r.Replace(tmpl))
}

func tail(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

func indent(s string) string {
	if s == "" {
		return ""
	}
	return "    " + strings.ReplaceAll(s, "\n", "\n    ")
}
