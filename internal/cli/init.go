package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/vora-sahil/opinionated/internal/fsx"
	"github.com/vora-sahil/opinionated/internal/infer"
	"github.com/vora-sahil/opinionated/internal/pack"
	"github.com/vora-sahil/opinionated/internal/structure"
)

const structureHeader = `# The folder layout of this repository, in one place.
# 'opine init' listed what exists. Nothing is enforced until you say more.
#
#   doc: what belongs here        shown to agents and in CONVENTIONS.md
#   role: a name for the folder   used by 'layers' below
#   closed: true                  no subdirectories other than those listed
#   files: kebab-case             naming for files below; 'dirs:' for directories
#   requires: [index.ts]          files every such folder must contain
#   "{feature}/":                 stands for any folder at that level
#   isolated: true                on a placeholder: its folders may not import each other
#
# layers:                 # highest first; imports may only point down
#   - interface
#   - feature | adapter   # '|' independent of each other, '+' may import each other
#   - core
# aliases:                # import prefixes that are not relative paths
#   "@/": src/

`

func cmdInit(e *env, args []string) int {
	fs := e.flags("init", "init [--defaults] [--force]")
	defaults := fs.Bool("defaults", false, "answer every decision that has a default, as for a new repository")
	force := fs.Bool("force", false, "overwrite an existing "+pack.Dir+"/"+pack.ConfigFile)
	if _, ok := parse(fs, args); !ok {
		return exitUsage
	}
	root := e.dir
	if out, err := exec.Command("git", "-C", e.dir, "rev-parse", "--show-toplevel").Output(); err == nil {
		root = strings.TrimSpace(string(out))
	}
	dir := filepath.Join(root, pack.Dir)
	cfgPath := filepath.Join(dir, pack.ConfigFile)
	if _, err := os.Stat(cfgPath); err == nil && !*force {
		e.errf("opine: %s already exists; edit it, or pass --force to start over\n", cfgPath)
		return exitUsage
	}
	fail := func(err error) int {
		e.errf("opine: %v\n", err)
		return exitUsage
	}

	files, err := fsx.List(root)
	if err != nil {
		return fail(err)
	}
	rep := infer.Analyze(root, files)
	newRepo := rep.SourceN == 0

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fail(err)
	}
	// A first pass with no decisions, to learn which decisions the packs offer.
	if err := os.WriteFile(cfgPath, []byte("version: 1\nextends:\n  - builtin:universal\n"), 0o644); err != nil {
		return fail(err)
	}
	res, err := pack.Load(root)
	if err != nil {
		return fail(err)
	}

	type choice struct{ value, note string }
	chosen := map[string]choice{}
	if t := rep.Tests; t != nil {
		n, of := t.Colocated, t.Colocated+t.Separate
		where := "sit beside the code"
		if t.Choice == "separate" {
			n, where = t.Separate, "live in a separate test tree"
		}
		chosen["layout/tests"] = choice{t.Choice, fmt.Sprintf("inferred: %d of %d test files %s", n, of, where)}
	}
	if n, of, ok := infer.Commits(root); ok {
		chosen["workflow/commits"] = choice{"conventional", fmt.Sprintf("inferred: %d of the last %d commits follow Conventional Commits", n, of)}
	}
	if *defaults || newRepo {
		for _, d := range res.Decisions {
			if _, ok := chosen[d.ID]; !ok && d.Default != "" {
				chosen[d.ID] = choice{d.Default, "the default for a new repository"}
			}
		}
	}

	var cfg strings.Builder
	cfg.WriteString("# opine configuration. https://github.com/vora-sahil/opinionated\nversion: 1\n\n")
	cfg.WriteString("# Layers beneath this repository's own conventions, lowest first.\n# Also: '- path: ../house-style' and '- git: <url>' with 'ref: <tag>'.\nextends:\n  - builtin:universal\n\n")
	cfg.WriteString("# The questions every codebase answers. 'opine decisions' explains each;\n# 'none' means you deliberately have no rule.\ndecisions:\n")
	undecided := 0
	for _, d := range res.Decisions {
		opts := strings.Join(append(append([]string{}, d.Options...), pack.NoRule), " | ")
		if c, ok := chosen[d.ID]; ok {
			fmt.Fprintf(&cfg, "  %s: %s    # %s\n", d.ID, c.value, c.note)
		} else {
			undecided++
			fmt.Fprintf(&cfg, "  # %s:    # %s  (%s)\n", d.ID, d.Question, opts)
		}
	}
	cfg.WriteString("\n# Change how an inherited convention applies here. A reason is required.\n# overrides:\n#   docs/readme: { level: off, reason: \"...\" }\n\n# Files no convention applies to.\n# ignore: [\"vendor/**\", \"**/*.generated.*\"]\n")
	if err := os.WriteFile(cfgPath, []byte(cfg.String()), 0o644); err != nil {
		return fail(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(pack.CacheDir+"/\n"), 0o644); err != nil {
		return fail(err)
	}

	var proposed []string
	for _, n := range rep.Naming {
		slug := strings.NewReplacer("#", "sharp", "+", "p").Replace(strings.ToLower(n.Language)) + "-files"
		p := filepath.Join(dir, "conventions", "naming", slug+".md")
		if _, err := os.Stat(p); err == nil {
			continue
		}
		var b strings.Builder
		b.WriteString("---\nstatus: proposed\nlevel: should\npaths:\n")
		for _, x := range n.Exts {
			fmt.Fprintf(&b, "  - \"**/*%s\"\n", x)
		}
		fmt.Fprintf(&b, "check:\n  kind: name\n  style: %s\nevidence:\n  follows: %d\n  violates: %d\n", n.Style, n.Follows, n.Violates)
		if len(n.Examples) > 0 {
			b.WriteString("  examples:\n")
			for _, x := range n.Examples {
				fmt.Fprintf(&b, "    - %q\n", x)
			}
		}
		title := fmt.Sprintf("Name %s files in %s", n.Language, n.Style)
		fmt.Fprintf(&b, "---\n# %s\n", title)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return fail(err)
		}
		if err := os.WriteFile(p, []byte(b.String()), 0o644); err != nil {
			return fail(err)
		}
		proposed = append(proposed, fmt.Sprintf("naming/%s   %s (%d of %d files already do)", slug, title, n.Follows, n.Follows+n.Violates))
	}

	structPath := filepath.Join(dir, structure.File)
	if _, err := os.Stat(structPath); err != nil {
		var b strings.Builder
		b.WriteString(structureHeader + "tree:\n")
		for _, d := range rep.TopDirs {
			fmt.Fprintf(&b, "  %s/:\n", d)
			for _, k := range rep.Tree[d] {
				fmt.Fprintf(&b, "    %s/:\n", k)
			}
		}
		if len(rep.TopDirs) == 0 {
			b.WriteString("  # src/:\n")
		}
		if err := os.WriteFile(structPath, []byte(b.String()), 0o644); err != nil {
			return fail(err)
		}
	}

	res, err = pack.Load(root)
	if err != nil {
		return fail(err)
	}

	if len(res.Problems) > 0 {
		e.printProblems(res.Problems)
		return exitUsage
	}

	e.out("Set up %s/ in %s\n\n", pack.Dir, root)
	if newRepo {
		e.out("No source files yet, so this is the baseline for a new repository.\n\n")
	} else {
		var langs []string
		for i, l := range rep.Languages {
			if i == 5 {
				break
			}
			langs = append(langs, fmt.Sprintf("%s (%d)", l.Name, l.N))
		}
		e.out("Found\n  %s %s\n", pad("languages", 12), strings.Join(langs, ", "))
		if len(rep.AgentFiles) > 0 {
			e.out("  %s %s\n  %s kept as they are; opine writes only inside its own markers\n", pad("agent files", 12), strings.Join(rep.AgentFiles, ", "), pad("", 12))
		}
		if len(rep.Linters) > 0 {
			e.out("  %s %s\n  %s kept; opine does not replace a linter, it covers what one cannot say\n", pad("linters", 12), strings.Join(rep.Linters, ", "), pad("", 12))
		}
		e.out("\n")
	}
	e.out("%s %s from builtin:universal\n", pad("Inherited", 10), plural(len(res.Active()), "convention"))
	for _, d := range res.Decisions {
		if c, ok := chosen[d.ID]; ok {
			e.out("%s %s = %s  (%s)\n", pad("Decided", 10), d.ID, c.value, c.note)
		}
	}
	for _, p := range proposed {
		e.out("%s %s\n", pad("Proposed", 10), p)
	}
	if undecided > 0 {
		e.out("%s %s; 'opine decisions' lists them\n", pad("Undecided", 10), plural(undecided, "decision"))
	}
	e.out("\nNext\n")
	if !newRepo {
		e.out("  opine prompt mine    instructions for your agent to propose the conventions this code already follows\n")
	}
	if len(proposed) > 0 || !newRepo {
		e.out("  opine review         say yes or no to what was proposed\n")
	}
	e.out("  opine check          see where the code stands\n  opine emit           write AGENTS.md, CLAUDE.md and CONVENTIONS.md\n")
	return exitOK
}
