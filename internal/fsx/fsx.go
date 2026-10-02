// Package fsx lists the files a repository actually owns and matches them
// against globs.
package fsx

import (
	"bytes"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

var alwaysSkip = map[string]bool{
	".git": true, "node_modules": true, ".opine/cache": true,
}

// walkSkip is used only outside git, where there is no ignore file to consult.
var walkSkip = map[string]bool{
	"vendor": true, "dist": true, "build": true, "target": true,
	".gradle": true, ".venv": true, "venv": true, "__pycache__": true, ".idea": true,
}

// List returns repo-relative, slash-separated paths of every file that is
// tracked or untracked-but-not-ignored. Outside git it walks the tree.
func List(root string) ([]string, error) {
	var files []string
	cmd := exec.Command("git", "-C", root, "ls-files", "-co", "--exclude-standard", "-z")
	if out, err := cmd.Output(); err == nil {
		for _, f := range bytes.Split(out, []byte{0}) {
			if len(f) == 0 {
				continue
			}
			p := string(f)
			if skipped(p) {
				continue
			}
			if st, err := os.Lstat(filepath.Join(root, filepath.FromSlash(p))); err != nil || !st.Mode().IsRegular() {
				continue
			}
			files = append(files, p)
		}
	} else {
		err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(root, p)
			rel = filepath.ToSlash(rel)
			if d.IsDir() {
				if rel != "." && (skipped(rel+"/x") || walkSkip[d.Name()]) {
					return filepath.SkipDir
				}
				return nil
			}
			if d.Type().IsRegular() {
				files = append(files, rel)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Strings(files)
	return files, nil
}

func skipped(p string) bool {
	for prefix := range alwaysSkip {
		if strings.HasPrefix(p, prefix+"/") || strings.Contains(p, "/"+prefix+"/") {
			return true
		}
	}
	return false
}

// Dirs returns every directory that contains at least one listed file,
// directly or below it.
func Dirs(files []string) []string {
	seen := map[string]bool{}
	for _, f := range files {
		for d := path.Dir(f); d != "." && d != "/" && !seen[d]; d = path.Dir(d) {
			seen[d] = true
		}
	}
	out := make([]string, 0, len(seen))
	for d := range seen {
		out = append(out, d)
	}
	sort.Strings(out)
	return out
}

// MatchAny reports whether p matches any pattern. Patterns use the same
// doublestar syntax agent rule files use, so a record's paths mean the same
// thing to opine and to the agent. A trailing slash means everything below.
func MatchAny(patterns []string, p string) bool {
	for _, pat := range patterns {
		if strings.HasSuffix(pat, "/") {
			pat += "**"
		}
		if ok, _ := doublestar.Match(pat, p); ok {
			return true
		}
	}
	return false
}

// InScope applies include and exclude globs. No include patterns means
// everything is included.
func InScope(include, exclude []string, p string) bool {
	if len(include) > 0 && !MatchAny(include, p) {
		return false
	}
	return !MatchAny(exclude, p)
}

// ValidPattern reports whether a glob is well formed.
func ValidPattern(pat string) bool { return doublestar.ValidatePattern(pat) }
