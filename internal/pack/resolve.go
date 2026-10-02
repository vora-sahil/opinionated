package pack

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/vora-sahil/opinionated/internal/fsx"
	"github.com/vora-sahil/opinionated/internal/record"
	"github.com/vora-sahil/opinionated/packs"
)

// LocalLayer is the name of the repository's own layer.
const LocalLayer = "this repo"

// NoRule is the answer to a decision that every repository may give: we
// considered this and deliberately have no convention.
const NoRule = "none"

// Decision is a question every codebase answers one way or another. Records
// that declare 'decides' are its options; only the chosen option is active.
type Decision struct {
	ID       string
	Question string `yaml:"question"`
	Default  string `yaml:"default"`
	Layer    string
	Options  []string
}

type Problem struct {
	Where string
	Msg   string
}

func (p Problem) String() string { return p.Where + ": " + p.Msg }

type layer struct {
	name      string
	records   []*record.Record
	decisions map[string]*Decision
}

// Resolved is the cascade after every layer, decision and override applies.
type Resolved struct {
	Root      string
	Config    *Config
	Layers    []string
	Chains    map[string][]*record.Record // every definition of an id, lowest layer first
	Records   []*record.Record            // the winning definition of each id, overrides applied
	Decisions []*Decision
	Problems  []Problem
}

// State says whether a record is in force, and if not, why.
func (r *Resolved) State(rec *record.Record) (active bool, why string) {
	switch rec.Status {
	case record.Proposed:
		return false, "proposed; run 'opine review'"
	case record.Rejected:
		return false, "rejected: " + rec.Reason
	case record.Deprecated:
		return false, "deprecated: " + rec.Reason
	}
	if rec.Decides != "" {
		chosen, ok := r.Config.Decisions[rec.Decides]
		if !ok {
			return false, fmt.Sprintf("undecided: %s is not set in %s", rec.Decides, ConfigFile)
		}
		if chosen != rec.Option {
			return false, fmt.Sprintf("not chosen: %s is %q", rec.Decides, chosen)
		}
	}
	if rec.Level == record.Off {
		if o, ok := r.Config.Overrides[rec.ID]; ok {
			return false, "off here: " + o.Reason
		}
		return false, "off"
	}
	return true, ""
}

// Active returns the records in force, sorted by id.
func (r *Resolved) Active() []*record.Record {
	var out []*record.Record
	for _, rec := range r.Records {
		if ok, _ := r.State(rec); ok {
			out = append(out, rec)
		}
	}
	return out
}

func (r *Resolved) Find(id string) *record.Record {
	for _, rec := range r.Records {
		if rec.ID == id {
			return rec
		}
	}
	return nil
}

func (r *Resolved) Decision(id string) *Decision {
	for _, d := range r.Decisions {
		if d.ID == id {
			return d
		}
	}
	return nil
}

// Load reads the configuration at root and resolves the cascade. Problems in
// the content are collected on the result; only an unreadable configuration
// is returned as an error.
func Load(root string) (*Resolved, error) {
	cfg, err := readConfig(root)
	if err != nil {
		return nil, err
	}
	res := &Resolved{Root: root, Config: cfg, Chains: map[string][]*record.Record{}}
	problem := func(where, f string, a ...any) {
		res.Problems = append(res.Problems, Problem{where, fmt.Sprintf(f, a...)})
	}

	var layers []*layer
	for _, e := range cfg.Extends {
		fsys, err := openExtend(root, e)
		if err != nil {
			problem(ConfigFile, "extends %s: %v", e.Name(), err)
			continue
		}
		l, probs := loadLayer(fsys, e.Name(), false)
		res.Problems = append(res.Problems, probs...)
		layers = append(layers, l)
	}
	local, probs := loadLayer(os.DirFS(filepath.Join(root, Dir)), LocalLayer, true)
	res.Problems = append(res.Problems, probs...)
	layers = append(layers, local)

	decisions := map[string]*Decision{}
	winners := map[string]*record.Record{}
	for _, l := range layers {
		res.Layers = append(res.Layers, l.name)
		for id, d := range l.decisions {
			decisions[id] = d
		}
		for _, rec := range l.records {
			res.Chains[rec.ID] = append(res.Chains[rec.ID], rec)
			winners[rec.ID] = rec
		}
	}

	for id, o := range cfg.Overrides {
		rec, ok := winners[id]
		where := ConfigFile + " overrides." + id
		if !ok {
			problem(where, "no convention with this id")
			continue
		}
		if strings.TrimSpace(o.Reason) == "" {
			problem(where, "an override needs a 'reason'")
		}
		c := *rec
		switch o.Level {
		case "":
		case record.Must, record.Should, record.Off:
			c.Level = o.Level
		default:
			problem(where, "level %q must be one of: must, should, off", o.Level)
		}
		if len(o.Paths) > 0 {
			c.Paths = o.Paths
		}
		c.Exclude = append(append([]string{}, rec.Exclude...), o.Exclude...)
		winners[id] = &c
	}

	for _, rec := range winners {
		res.Records = append(res.Records, rec)
		if rec.Decides == "" {
			continue
		}
		d, ok := decisions[rec.Decides]
		if !ok {
			problem(rec.Source, "decides %q, which no layer declares in decisions.yaml", rec.Decides)
			continue
		}
		d.Options = append(d.Options, rec.Option)
	}
	sort.Slice(res.Records, func(i, j int) bool { return res.Records[i].ID < res.Records[j].ID })

	for id, d := range decisions {
		d.Options = uniqueSorted(d.Options)
		res.Decisions = append(res.Decisions, d)
		if d.Default != "" && !contains(d.Options, d.Default) {
			problem("decisions.yaml "+id, "default %q is not one of its options", d.Default)
		}
	}
	sort.Slice(res.Decisions, func(i, j int) bool { return res.Decisions[i].ID < res.Decisions[j].ID })

	for id, opt := range cfg.Decisions {
		d, ok := decisions[id]
		where := ConfigFile + " decisions." + id
		switch {
		case !ok:
			problem(where, "no such decision; run 'opine decisions' to list them")
		case opt != NoRule && !contains(d.Options, opt):
			problem(where, "%q is not an option; options are: %s, %s", opt, strings.Join(d.Options, ", "), NoRule)
		}
	}
	for _, g := range cfg.Ignore {
		if !fsx.ValidPattern(g) {
			problem(ConfigFile+" ignore", "bad glob %q", g)
		}
	}
	sort.SliceStable(res.Problems, func(i, j int) bool { return res.Problems[i].Where < res.Problems[j].Where })
	return res, nil
}

func openExtend(root string, e Extend) (fs.FS, error) {
	switch {
	case e.Builtin != "":
		sub, err := fs.Sub(packs.FS, e.Builtin)
		if err != nil {
			return nil, err
		}
		if _, err := fs.Stat(sub, "pack.yaml"); err != nil {
			return nil, fmt.Errorf("no builtin pack named %q", e.Builtin)
		}
		return sub, nil
	case e.Path != "":
		p := e.Path
		if !filepath.IsAbs(p) {
			p = filepath.Join(root, p)
		}
		return packRoot(p)
	case e.Git != "":
		dir := CachePath(root, e)
		if _, err := os.Stat(dir); err != nil {
			return nil, fmt.Errorf("not fetched; run 'opine sync'")
		}
		if err := verifyLock(root, e, dir); err != nil {
			return nil, err
		}
		return packRoot(dir)
	}
	return nil, fmt.Errorf("empty entry")
}

// packRoot accepts either a dedicated pack (pack.yaml at its root) or any
// repository that uses opine, whose .opine directory then serves as the pack.
func packRoot(dir string) (fs.FS, error) {
	if _, err := os.Stat(filepath.Join(dir, "pack.yaml")); err == nil {
		return os.DirFS(dir), nil
	}
	if _, err := os.Stat(filepath.Join(dir, Dir, "conventions")); err == nil {
		return os.DirFS(filepath.Join(dir, Dir)), nil
	}
	return nil, fmt.Errorf("%s is not a pack: it has neither pack.yaml nor %s/conventions", dir, Dir)
}

func loadLayer(fsys fs.FS, name string, local bool) (*layer, []Problem) {
	l := &layer{name: name, decisions: map[string]*Decision{}}
	var probs []Problem
	where := func(p string) string {
		if local {
			return path.Join(Dir, p)
		}
		return name + ": " + p
	}

	if data, err := fs.ReadFile(fsys, "decisions.yaml"); err == nil {
		var ds map[string]*Decision
		if err := yaml.Unmarshal(data, &ds); err != nil {
			probs = append(probs, Problem{where("decisions.yaml"), err.Error()})
		}
		for id, d := range ds {
			if d == nil || strings.TrimSpace(d.Question) == "" {
				probs = append(probs, Problem{where("decisions.yaml"), id + ": needs a 'question'"})
				continue
			}
			d.ID, d.Layer = id, name
			l.decisions[id] = d
		}
	}

	_ = fs.WalkDir(fsys, "conventions", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".md") {
			return nil
		}
		rel := strings.TrimSuffix(strings.TrimPrefix(p, "conventions/"), ".md")
		data, err := fs.ReadFile(fsys, p)
		if err != nil {
			probs = append(probs, Problem{where(p), err.Error()})
			return nil
		}
		rec, err := record.Parse(rel, where(p), data)
		if err != nil {
			probs = append(probs, Problem{where(p), err.Error()})
			return nil
		}
		rec.Layer, rec.Local = name, local
		for _, msg := range record.Lint(rec) {
			probs = append(probs, Problem{where(p), msg})
		}
		for _, g := range append(append([]string{}, rec.Paths...), rec.Exclude...) {
			if !fsx.ValidPattern(g) {
				probs = append(probs, Problem{where(p), fmt.Sprintf("bad glob %q", g)})
			}
		}
		if !local && rec.Check != nil && rec.Check.Kind == "run" {
			probs = append(probs, Problem{where(p), "a pack may not run commands; 'run' checks are only allowed in a repository's own conventions"})
			rec.Check = nil
			rec.Review = "Enforced by a command this pack is not allowed to run."
		}
		l.records = append(l.records, rec)
		return nil
	})
	return l, probs
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func uniqueSorted(in []string) []string {
	sort.Strings(in)
	var out []string
	for i, s := range in {
		if i == 0 || s != in[i-1] {
			out = append(out, s)
		}
	}
	return out
}
