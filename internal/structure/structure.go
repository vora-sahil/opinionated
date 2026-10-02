// Package structure reads .opine/structure.yaml, the one place a repository
// states its folder layout, and checks the tree against it.
//
// The file answers three separate questions, each borrowed from a tool that
// already answers it well for one ecosystem:
//
//	shape      what may exist where          (ls-lint, eslint-plugin-project-structure)
//	roles      what each folder is for       (Nx tags, Feature-Sliced Design)
//	direction  which roles may import which  (import-linter, dependency-cruiser)
package structure

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/vora-sahil/opinionated/internal/check"
	"github.com/vora-sahil/opinionated/internal/pack"
	"github.com/vora-sahil/opinionated/internal/record"
)

const File = "structure.yaml"

// Convention ids that structure findings are reported under.
const (
	TreeID   = "layout/structure"
	NameID   = "naming/structure"
	LayersID = "boundaries/layers"
)

type Node struct {
	Name        string // directory name, or the placeholder's name
	Placeholder bool   // "{feature}/": stands for any directory at this level
	Doc         string
	Role        string
	Closed      bool // no subdirectories other than the ones declared
	Isolated    bool // instances of this placeholder may not import each other
	Files       string
	Dirs        string
	Requires    []string
	Children    []*Node
}

type Spec struct {
	Level   record.Level
	Tree    *Node
	Layers  [][][]string // layers high to low; each a list of groups; each group a list of roles
	Aliases map[string]string

	rawLayers []string
}

var nodeStyles = map[string]bool{"kebab-case": true, "snake_case": true, "camelCase": true, "PascalCase": true}

// Load reads the structure file. A repository without one has no structure
// rules, which is not an error.
func Load(root string) (*Spec, []string, error) {
	data, err := os.ReadFile(filepath.Join(root, pack.Dir, File))
	if os.IsNotExist(err) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	return Parse(data)
}

func Parse(data []byte) (*Spec, []string, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, nil, fmt.Errorf("%s: %w", File, err)
	}
	s := &Spec{Level: record.Must, Tree: &Node{}, Aliases: map[string]string{}}
	if len(doc.Content) == 0 {
		return s, nil, nil
	}
	top := doc.Content[0]
	if top.Kind != yaml.MappingNode {
		return nil, nil, fmt.Errorf("%s: must be a mapping", File)
	}
	var probs []string
	bad := func(f string, a ...any) { probs = append(probs, fmt.Sprintf(f, a...)) }

	for i := 0; i+1 < len(top.Content); i += 2 {
		k, v := top.Content[i].Value, top.Content[i+1]
		switch k {
		case "level":
			s.Level = record.Level(v.Value)
			if s.Level != record.Must && s.Level != record.Should {
				bad("level %q must be must or should", v.Value)
			}
		case "tree":
			parseNode(s.Tree, v, "", bad)
		case "layers":
			if err := v.Decode(&s.rawLayers); err != nil {
				bad("layers must be a list of strings, highest layer first; write 'a | b' for independent roles and 'a + b' for roles that may import each other")
			}
		case "aliases":
			if err := v.Decode(&s.Aliases); err != nil {
				bad("aliases must map an import prefix to a path")
			}
		default:
			bad("unknown key %q; keys are: tree, layers, aliases, level", k)
		}
	}

	roles := map[string]bool{}
	var collect func(n *Node)
	collect = func(n *Node) {
		if n.Role != "" {
			roles[n.Role] = true
		}
		for _, c := range n.Children {
			collect(c)
		}
	}
	collect(s.Tree)
	seen := map[string]bool{}
	for _, raw := range s.rawLayers {
		var layer [][]string
		for _, g := range strings.Split(raw, "|") {
			var group []string
			for _, r := range strings.Split(g, "+") {
				r = strings.TrimSpace(r)
				switch {
				case r == "":
					bad("layers: empty role in %q", raw)
					continue
				case !roles[r]:
					bad("layers: role %q is not assigned to any directory in the tree", r)
				case seen[r]:
					bad("layers: role %q appears twice", r)
				}
				seen[r] = true
				group = append(group, r)
			}
			layer = append(layer, group)
		}
		s.Layers = append(s.Layers, layer)
	}
	return s, probs, nil
}

func parseNode(n *Node, v *yaml.Node, at string, bad func(string, ...any)) {
	if v.Kind == yaml.ScalarNode && (v.Tag == "!!null" || v.Value == "") {
		return
	}
	if v.Kind != yaml.MappingNode {
		bad("%s: expected a mapping", orRoot(at))
		return
	}
	placeholders := 0
	for i := 0; i+1 < len(v.Content); i += 2 {
		k, val := v.Content[i].Value, v.Content[i+1]
		if strings.HasSuffix(k, "/") {
			name := strings.TrimSuffix(k, "/")
			child := &Node{Name: name}
			if strings.HasPrefix(name, "{") && strings.HasSuffix(name, "}") {
				child.Placeholder = true
				if placeholders++; placeholders > 1 {
					bad("%s: only one placeholder directory is allowed per level", orRoot(at))
				}
			} else if name == "" || strings.Contains(name, "/") {
				bad("%s: %q must be a single directory name; nest deeper levels", orRoot(at), k)
				continue
			}
			parseNode(child, val, at+k, bad)
			n.Children = append(n.Children, child)
			continue
		}
		switch k {
		case "doc":
			n.Doc = strings.TrimSpace(val.Value)
		case "role":
			n.Role = val.Value
		case "closed":
			n.Closed = val.Value == "true"
		case "isolated":
			n.Isolated = val.Value == "true"
		case "files", "dirs":
			if !nodeStyles[val.Value] {
				bad("%s: %s %q must be one of: kebab-case, snake_case, camelCase, PascalCase", orRoot(at), k, val.Value)
			} else if k == "files" {
				n.Files = val.Value
			} else {
				n.Dirs = val.Value
			}
		case "requires":
			var l record.StringList
			if err := val.Decode(&l); err != nil {
				bad("%s: requires must be a list of paths", orRoot(at))
			}
			n.Requires = l
		default:
			bad("%s: unknown key %q; directories end with '/', attributes are: doc, role, closed, isolated, files, dirs, requires", orRoot(at), k)
		}
	}
	if n.Isolated && !n.Placeholder {
		bad("%s: isolated only applies to a placeholder directory such as {feature}/", orRoot(at))
	}
}

func orRoot(at string) string {
	if at == "" {
		return "tree"
	}
	return at
}

// RoleDir is a real directory that the tree assigns a role to.
type RoleDir struct {
	Path     string
	Role     string
	Isolated bool
}

// Layout is the spec matched against the repository's actual directories.
type Layout struct {
	Spec     *Spec
	RoleDirs []RoleDir
}

type inherit struct{ files, dirs string }

// Check matches the tree against the repository and reports shape and naming
// findings. Layer findings come from CheckImports.
func (s *Spec) Check(files []string) (*Layout, []check.Violation) {
	children := map[string][]string{} // dir -> sorted child dir names
	direct := map[string][]string{}   // dir -> files directly in it
	exists := map[string]bool{}
	seenDir := map[string]bool{}
	for _, f := range files {
		exists[f] = true
		d := path.Dir(f)
		direct[d] = append(direct[d], path.Base(f))
		for ; d != "."; d = path.Dir(d) {
			if seenDir[d] {
				break
			}
			seenDir[d] = true
			exists[d] = true
			parent := path.Dir(d)
			children[parent] = append(children[parent], path.Base(d))
		}
	}
	for _, c := range children {
		sort.Strings(c)
	}

	lay := &Layout{Spec: s}
	var out []check.Violation
	add := func(id, file, msg string) {
		out = append(out, check.Violation{ID: id, Level: s.Level, File: file, Message: msg})
	}

	var walk func(n *Node, dir string, in inherit)
	walk = func(n *Node, dir string, in inherit) {
		if n != nil {
			if n.Files != "" {
				in.files = n.Files
			}
			if n.Dirs != "" {
				in.dirs = n.Dirs
			}
			if n.Role != "" {
				lay.RoleDirs = append(lay.RoleDirs, RoleDir{Path: dir, Role: n.Role, Isolated: n.Isolated})
			}
			for _, req := range n.Requires {
				p := path.Join(dir, req)
				if !exists[p] {
					add(TreeID, dir, fmt.Sprintf("%s/ must contain %s", dir, req))
				}
			}
		}
		if in.files != "" {
			for _, base := range direct[dir] {
				if stem := check.Stem(base); stem != "" && !check.StyleMatch(in.files, stem) {
					add(NameID, path.Join(dir, base), fmt.Sprintf("file name %q is not %s", base, in.files))
				}
			}
		}
		for _, name := range children[dir] {
			childPath := path.Join(dir, name)
			var child *Node
			literal := false
			if n != nil {
				var ph *Node
				for _, c := range n.Children {
					if c.Placeholder {
						ph = c
					} else if c.Name == name {
						child, literal = c, true
					}
				}
				if child == nil {
					child = ph
				}
				if child == nil && n.Closed && !strings.HasPrefix(name, ".") {
					add(TreeID, childPath, fmt.Sprintf("%s/ is not part of the declared structure; %s allows: %s",
						childPath, displayDir(dir), allowed(n)))
					continue
				}
			}
			if !literal && in.dirs != "" && !strings.HasPrefix(name, ".") && !check.StyleMatch(in.dirs, strings.Trim(name, "_")) {
				add(NameID, childPath, fmt.Sprintf("directory name %q is not %s", name, in.dirs))
			}
			walk(child, childPath, in)
		}
	}
	walk(s.Tree, ".", inherit{})
	sort.Slice(lay.RoleDirs, func(i, j int) bool { return lay.RoleDirs[i].Path < lay.RoleDirs[j].Path })
	check.Sort(out)
	return lay, out
}

func displayDir(d string) string {
	if d == "." {
		return "the repository root"
	}
	return d + "/"
}

func allowed(n *Node) string {
	var names []string
	for _, c := range n.Children {
		if c.Placeholder {
			names = append(names, "any "+c.Name)
		} else {
			names = append(names, c.Name+"/")
		}
	}
	if len(names) == 0 {
		return "no subdirectories"
	}
	return strings.Join(names, ", ")
}

// RoleOf returns the role directory that owns a path, if any.
func (l *Layout) RoleOf(p string) (RoleDir, bool) {
	best, ok := RoleDir{}, false
	for _, rd := range l.RoleDirs {
		if (p == rd.Path || strings.HasPrefix(p, rd.Path+"/")) && len(rd.Path) >= len(best.Path) {
			best, ok = rd, true
		}
	}
	return best, ok
}

// position locates a role in the layers: its layer index and group index.
func (s *Spec) position(role string) (layer, group int, ok bool) {
	for li, l := range s.Layers {
		for gi, g := range l {
			for _, r := range g {
				if r == role {
					return li, gi, true
				}
			}
		}
	}
	return 0, 0, false
}

// LayerSummary renders the dependency direction on one line.
func (s *Spec) LayerSummary() string {
	if len(s.rawLayers) == 0 {
		return ""
	}
	parts := make([]string, len(s.rawLayers))
	for i, l := range s.rawLayers {
		parts[i] = strings.Join(strings.Fields(l), " ")
	}
	return strings.Join(parts, " → ")
}

// Guide lists the directories that carry a doc, in tree order: where code
// goes, stated as instructions rather than as an overview.
func (s *Spec) Guide() []string {
	var out []string
	var walk func(n *Node, dir string)
	walk = func(n *Node, dir string) {
		for _, c := range n.Children {
			name := c.Name
			if c.Placeholder {
				name = "<" + strings.Trim(c.Name, "{}") + ">"
			}
			p := dir + name + "/"
			if c.Doc != "" {
				out = append(out, fmt.Sprintf("`%s` — %s", p, c.Doc))
			}
			walk(c, p)
		}
	}
	walk(s.Tree, "")
	return out
}

// Render draws the declared tree the way a README would.
func (s *Spec) Render() string {
	var b strings.Builder
	var walk func(n *Node, depth int)
	walk = func(n *Node, depth int) {
		for _, c := range n.Children {
			name := c.Name
			if c.Placeholder {
				name = "<" + strings.Trim(c.Name, "{}") + ">"
			}
			line := strings.Repeat("  ", depth) + name + "/"
			var notes []string
			if c.Role != "" {
				notes = append(notes, "["+c.Role+"]")
			}
			if c.Doc != "" {
				notes = append(notes, c.Doc)
			}
			if c.Closed {
				notes = append(notes, "(closed)")
			}
			if len(notes) > 0 {
				line = fmt.Sprintf("%-24s %s", line, strings.Join(notes, " "))
			}
			b.WriteString(strings.TrimRight(line, " ") + "\n")
			walk(c, depth+1)
		}
	}
	walk(s.Tree, 0)
	return b.String()
}
