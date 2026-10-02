package cli

import (
	"fmt"
	"strings"

	"github.com/vora-sahil/opinionated/internal/emit"
	"github.com/vora-sahil/opinionated/internal/pack"
	"github.com/vora-sahil/opinionated/internal/record"
	"github.com/vora-sahil/opinionated/internal/structure"
)

func enforcement(r *record.Record) string {
	if r.Mechanical() {
		return "checked"
	}
	return "review"
}

func cmdList(e *env, args []string) int {
	fs := e.flags("list", "list [--all]")
	all := fs.Bool("all", false, "include conventions that are not in force, with the reason")
	if _, ok := parse(fs, args); !ok {
		return exitUsage
	}
	res, _, ok := e.load()
	if !ok {
		return exitUsage
	}
	n := 0
	for _, r := range res.Records {
		active, why := res.State(r)
		if !active && !*all {
			continue
		}
		level := string(r.Level)
		if !active {
			level = "-"
		}
		line := fmt.Sprintf("%s  %s  %s", pad(level, 6), pad(r.ID, 36), r.Title)
		if active {
			n++
			line += fmt.Sprintf("  [%s, %s]", enforcement(r), r.Layer)
		} else {
			line += "  [" + why + "]"
		}
		e.out("%s\n", line)
	}
	e.out("\n%s in force.\n", plural(n, "convention"))
	return exitOK
}

var structureIDs = map[string]string{
	structure.TreeID:   "Directories match the tree declared in " + pack.Dir + "/" + structure.File + ".",
	structure.NameID:   "File and directory names follow the styles declared in " + pack.Dir + "/" + structure.File + ".",
	structure.LayersID: "Imports follow the layers declared in " + pack.Dir + "/" + structure.File + ".",
}

func cmdExplain(e *env, args []string) int {
	fs := e.flags("explain", "explain <id>")
	pos, ok := parse(fs, args)
	if !ok || len(pos) != 1 {
		fs.Usage()
		return exitUsage
	}
	id := pos[0]
	res, spec, ok := e.load()
	if !ok {
		return exitUsage
	}
	if desc, ok := structureIDs[id]; ok {
		e.out("%s\n\n  %s\n", id, desc)
		if spec != nil {
			e.out("\n%s", indentBlock(spec.Render(), "  "))
			if l := spec.LayerSummary(); l != "" {
				e.out("\n  Imports only point down these layers: %s\n", l)
			}
		}
		e.out("\n%s", suppressHelp(id))
		return exitOK
	}
	r := res.Find(id)
	if r == nil {
		e.errf("opine: no convention %q; 'opine list --all' shows every id\n", id)
		return exitUsage
	}
	active, why := res.State(r)
	state := "in force"
	if !active {
		state = why
	}
	e.out("%s  ·  %s  ·  %s\n\n  %s\n", r.ID, r.Level, state, r.Title)
	if r.Why != "" {
		e.out("\n  %s\n", r.Why)
	}
	for _, set := range []struct {
		label string
		ex    []record.Example
	}{{"Bad", r.Bad}, {"Good", r.Good}} {
		for _, x := range set.ex {
			e.out("\n  %s:\n%s", set.label, indentBlock(x.Code+"\n", "      "))
		}
	}
	e.out("\n")
	row := func(k, v string) { e.out("%s %s\n", pad(k, 12), v) }
	if len(r.Paths) > 0 {
		scope := strings.Join(r.Paths, ", ")
		if len(r.Exclude) > 0 {
			scope += "   except " + strings.Join(r.Exclude, ", ")
		}
		row("Applies to", scope)
	} else {
		row("Applies to", "the whole repository")
	}
	if c := r.Check; c != nil {
		row("Enforced by", "opine check ("+describeCheck(c)+")")
	} else {
		row("Enforced by", "review: "+r.Review)
	}
	chain := res.Chains[r.ID]
	for i := len(chain) - 1; i >= 0; i-- {
		label := "Defined in"
		if i < len(chain)-1 {
			label = "  replaces"
		}
		row(label, chain[i].Layer+"  "+chain[i].Source)
	}
	if o, ok := res.Config.Overrides[r.ID]; ok {
		var parts []string
		if o.Level != "" {
			parts = append(parts, "level "+string(o.Level))
		}
		if len(o.Paths) > 0 {
			parts = append(parts, "paths "+strings.Join(o.Paths, ", "))
		}
		if len(o.Exclude) > 0 {
			parts = append(parts, "exclude "+strings.Join(o.Exclude, ", "))
		}
		row("Overridden", strings.Join(parts, "; ")+"  because: "+o.Reason)
	}
	if r.Decides != "" {
		row("Decision", r.Decides+" = "+r.Option)
	}
	if len(r.See) > 0 {
		row("See", strings.Join(r.See, ", "))
	}
	if r.Mechanical() {
		e.out("\n%s", suppressHelp(r.ID))
	}
	return exitOK
}

func suppressHelp(id string) string {
	return fmt.Sprintf("To allow one exception, comment on or above the line:\n  opine-ignore %s: <why this case is different>\n", id)
}

func describeCheck(c *record.Check) string {
	switch c.Kind {
	case "forbid":
		if c.Unless != "" {
			return "forbid /" + c.Pattern + "/ unless /" + c.Unless + "/"
		}
		return "forbid /" + c.Pattern + "/"
	case "require":
		return "require /" + c.Pattern + "/"
	case "name":
		t := c.Target
		if t == "" {
			t = "file"
		}
		return t + " names in " + c.Style
	case "pair":
		return "needs " + strings.Join(c.Needs, " or ")
	case "exists", "absent":
		return c.Kind + " " + strings.Join(c.Files, ", ")
	case "max-lines":
		return fmt.Sprintf("at most %d lines", c.Max)
	case "run":
		return "$ " + c.Cmd
	}
	return c.Kind
}

func indentBlock(s, prefix string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i, l := range lines {
		if l != "" {
			lines[i] = prefix + l
		}
	}
	return strings.Join(lines, "\n") + "\n"
}

func cmdDecisions(e *env, args []string) int {
	fs := e.flags("decisions", "decisions")
	if _, ok := parse(fs, args); !ok {
		return exitUsage
	}
	res, _, ok := e.load()
	if !ok {
		return exitUsage
	}
	undecided := 0
	for _, d := range res.Decisions {
		options := strings.Join(append(append([]string{}, d.Options...), pack.NoRule), " | ")
		choice, ok := res.Config.Decisions[d.ID]
		if !ok {
			choice = "undecided"
			undecided++
		}
		e.out("%s  %s\n%s  = %s    (%s)\n\n", pad(d.ID, 22), d.Question, pad("", 22), choice, options)
	}
	if undecided > 0 {
		e.out("%s undecided. Set them under 'decisions:' in %s/%s.\n", plural(undecided, "decision"), pack.Dir, pack.ConfigFile)
	}
	return exitOK
}

func cmdLint(e *env, args []string) int {
	fs := e.flags("lint", "lint")
	if _, ok := parse(fs, args); !ok {
		return exitUsage
	}
	res, _, ok := e.load()
	if !ok {
		return exitViolations
	}
	e.out("%s valid across %s.\n", plural(len(res.Records), "convention"), plural(len(res.Layers), "layer"))
	return exitOK
}

func cmdSync(e *env, args []string) int {
	fs := e.flags("sync", "sync [--update]")
	update := fs.Bool("update", false, "re-resolve each ref instead of using the pinned commit")
	if _, ok := parse(fs, args); !ok {
		return exitUsage
	}
	root, err := pack.FindRoot(e.dir)
	if err != nil {
		e.errf("opine: %v\n", err)
		return exitUsage
	}
	if err := pack.Sync(root, *update, func(f string, a ...any) { e.out(f+"\n", a...) }); err != nil {
		e.errf("opine: %v\n", err)
		return exitUsage
	}
	return exitOK
}

func cmdEmit(e *env, args []string) int {
	fs := e.flags("emit", "emit [--check]")
	checkOnly := fs.Bool("check", false, "write nothing; fail if the emitted files are out of date")
	if _, ok := parse(fs, args); !ok {
		return exitUsage
	}
	res, spec, ok := e.load()
	if !ok {
		return exitUsage
	}
	plan, err := emit.Build(res, spec)
	if err != nil {
		e.errf("opine: %v\n", err)
		return exitUsage
	}
	for _, w := range plan.Warnings {
		e.errf("warning: %s\n", w)
	}
	if *checkOnly {
		stale := plan.Stale(res.Root)
		if len(stale) == 0 {
			e.out("Emitted files are up to date.\n")
			return exitOK
		}
		e.errf("Out of date; run 'opine emit':\n")
		for _, s := range stale {
			e.errf("  %s\n", s)
		}
		return exitViolations
	}
	changed, err := plan.Write(res.Root)
	if err != nil {
		e.errf("opine: %v\n", err)
		return exitUsage
	}
	if len(changed) == 0 {
		e.out("Nothing to do; emitted files are up to date.\n")
		return exitOK
	}
	for _, c := range changed {
		e.out("  %s\n", c)
	}
	e.out("Wrote %s.\n", plural(len(changed), "file"))
	return exitOK
}
