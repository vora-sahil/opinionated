package structure

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/vora-sahil/opinionated/internal/check"
)

type language struct {
	patterns  []*regexp.Regexp
	qualified bool   // imports are fully-qualified names, matched by package path
	sep       string // separator in qualified names
}

func re(s string) *regexp.Regexp { return regexp.MustCompile(s) }

var (
	jsLang = &language{patterns: []*regexp.Regexp{
		re(`\bfrom\s+['"]([^'"]+)['"]`),
		re(`^\s*import\s+['"]([^'"]+)['"]`),
		re(`\brequire\(\s*['"]([^'"]+)['"]\s*\)`),
		re(`\bimport\(\s*['"]([^'"]+)['"]\s*\)`),
	}}
	jvmLang = &language{qualified: true, sep: ".", patterns: []*regexp.Regexp{
		re(`^\s*import\s+(?:static\s+)?([\w.]+)`),
	}}
	cLang = &language{patterns: []*regexp.Regexp{re(`^\s*#\s*include\s+"([^"]+)"`)}}

	languages = map[string]*language{
		".js": jsLang, ".jsx": jsLang, ".ts": jsLang, ".tsx": jsLang, ".mjs": jsLang, ".cjs": jsLang,
		".vue": jsLang, ".svelte": jsLang,
		".py": {qualified: true, sep: ".", patterns: []*regexp.Regexp{
			re(`^\s*from\s+(\.*[\w.]*)\s+import\b`),
			re(`^\s*import\s+([\w.]+)`),
		}},
		".java": jvmLang, ".kt": jvmLang, ".kts": jvmLang, ".scala": jvmLang, ".groovy": jvmLang,
		".go": {patterns: []*regexp.Regexp{re(`^\s*(?:import\s+)?(?:[\w.]+\s+)?"([^"]+)"\s*$`)}},
		".rs": {qualified: true, sep: "::", patterns: []*regexp.Regexp{
			re(`^\s*(?:pub(?:\([^)]*\))?\s+)?use\s+([\w:]+)`),
		}},
		".cs": {qualified: true, sep: ".", patterns: []*regexp.Regexp{
			re(`^\s*(?:global\s+)?using\s+(?:static\s+)?([\w.]+)\s*;`),
		}},
		".php": {qualified: true, sep: `\`, patterns: []*regexp.Regexp{re(`^\s*use\s+([\w\\]+)`)}},
		".rb":  {patterns: []*regexp.Regexp{re(`^\s*require_relative\s+['"]([^'"]+)['"]`)}},
		".c":   cLang, ".h": cLang, ".cc": cLang, ".cpp": cLang, ".hpp": cLang,
	}
)

// CheckImports enforces the declared layers and isolation. only, when not
// empty, limits the scan to those files.
//
// Resolution is exact for relative imports, aliases and Go module paths. A
// fully-qualified name (Java, Kotlin, Python, C#, PHP, Rust) is matched to a
// role directory when the name begins with the trailing segments of that
// directory's path, so com.acme.infra.Db resolves to .../com/acme/infra.
// Anything that resolves to no role directory is treated as external.
func (l *Layout) CheckImports(src *check.Source, files, only []string) []check.Violation {
	s := l.Spec
	if len(s.Layers) == 0 && !l.anyIsolated() {
		return nil
	}
	restrict := map[string]bool{}
	for _, f := range only {
		restrict[f] = true
	}
	module := goModule(src.Root)

	aliases := make([]string, 0, len(s.Aliases))
	for a := range s.Aliases {
		aliases = append(aliases, a)
	}
	sort.Slice(aliases, func(i, j int) bool { return len(aliases[i]) > len(aliases[j]) })

	var out []check.Violation
	for _, f := range files {
		if len(restrict) > 0 && !restrict[f] {
			continue
		}
		lang := languages[path.Ext(f)]
		if lang == nil {
			continue
		}
		from, ok := l.RoleOf(f)
		if !ok {
			continue
		}
		for i, line := range src.Lines(f) {
			for _, pat := range lang.patterns {
				m := pat.FindStringSubmatch(line)
				if m == nil {
					continue
				}
				to, ok := l.resolve(f, m[1], lang, module, aliases)
				if !ok || to.Path == from.Path {
					continue
				}
				if msg := l.verdict(from, to); msg != "" {
					out = append(out, check.Violation{ID: LayersID, Level: s.Level, File: f, Line: i + 1, Message: msg})
				}
				break
			}
		}
	}
	check.Sort(out)
	return out
}

func (l *Layout) anyIsolated() bool {
	for _, rd := range l.RoleDirs {
		if rd.Isolated {
			return true
		}
	}
	return false
}

func (l *Layout) verdict(from, to RoleDir) string {
	s := l.Spec
	if from.Role == to.Role {
		if from.Isolated {
			return fmt.Sprintf("%s/ must not import its sibling %s/: each %s is isolated", from.Path, to.Path, from.Role)
		}
		return ""
	}
	fl, fg, fok := s.position(from.Role)
	tl, tg, tok := s.position(to.Role)
	if !fok || !tok {
		return ""
	}
	switch {
	case tl > fl:
		return ""
	case tl < fl:
		return fmt.Sprintf("%s must not import %s: imports only point down the layers (%s)", from.Role, to.Role, s.LayerSummary())
	case fg == tg:
		return ""
	default:
		return fmt.Sprintf("%s must not import %s: they are independent (%s)", from.Role, to.Role, s.LayerSummary())
	}
}

func (l *Layout) resolve(file, spec string, lang *language, module string, aliases []string) (RoleDir, bool) {
	dir := path.Dir(file)
	for _, a := range aliases {
		if strings.HasPrefix(spec, a) {
			return l.RoleOf(path.Clean(path.Join(l.Spec.Aliases[a], strings.TrimPrefix(spec, a))))
		}
	}
	switch {
	case strings.HasPrefix(spec, "./") || strings.HasPrefix(spec, "../"):
		return l.RoleOf(path.Join(dir, spec))
	case lang == cLang || path.Ext(file) == ".rb":
		return l.RoleOf(path.Join(dir, spec))
	case path.Ext(file) == ".py" && strings.HasPrefix(spec, "."):
		rest := strings.TrimLeft(spec, ".")
		base := dir
		for i := 1; i < len(spec)-len(rest); i++ {
			base = path.Dir(base)
		}
		return l.RoleOf(path.Join(base, strings.ReplaceAll(rest, ".", "/")))
	case path.Ext(file) == ".go":
		if module != "" && strings.HasPrefix(spec, module+"/") {
			return l.RoleOf(strings.TrimPrefix(spec, module+"/"))
		}
		return RoleDir{}, false
	case !lang.qualified:
		return RoleDir{}, false
	}

	segs := strings.Split(spec, lang.sep)
	if lang.sep == "::" {
		if segs[0] != "crate" {
			return RoleDir{}, false
		}
		segs = segs[1:]
	}
	best, bestK := RoleDir{}, 0
	for _, rd := range l.RoleDirs {
		dirSegs := strings.Split(rd.Path, "/")
		for k := len(dirSegs); k > bestK; k-- {
			if k <= len(segs) && equal(dirSegs[len(dirSegs)-k:], segs[:k]) {
				best, bestK = rd, k
				break
			}
		}
	}
	return best, bestK > 0
}

func equal(a, b []string) bool {
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func goModule(root string) string {
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "module "); ok {
			return strings.TrimSpace(rest)
		}
	}
	return ""
}
