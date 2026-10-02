// Package cli implements the opine command line.
package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/vora-sahil/opinionated/internal/pack"
	"github.com/vora-sahil/opinionated/internal/structure"
)

// Version is set at build time.
var Version = "dev"

// Exit codes. Violations and bad input are different failures and a script
// may want to tell them apart.
const (
	exitOK         = 0
	exitViolations = 1
	exitUsage      = 2
)

type env struct {
	dir    string
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
}

func (e *env) out(f string, a ...any)  { fmt.Fprintf(e.stdout, f, a...) }
func (e *env) errf(f string, a ...any) { fmt.Fprintf(e.stderr, f, a...) }

const usage = `opine keeps a codebase's conventions written down, enforced, and in front of
every agent and person who works on it.

Usage: opine <command> [flags]

Start
  init        Set up .opine/ from what this repository already does
  prompt      Print instructions for your agent: 'mine' conventions from the
              code, or 'review' a change against the ones no machine can check
  review      Accept or reject proposed conventions, one at a time

Daily
  check       Verify the code follows the conventions (the CI gate)
  emit        Write AGENTS.md, CLAUDE.md, rule files and CONVENTIONS.md
  explain     Show one convention: rule, reason, example, where it came from

Inspect
  list        List the conventions in force
  decisions   Show the decisions and what this repository chose
  lint        Validate everything under .opine/
  sync        Fetch the git packs named in opine.yaml

Run 'opine <command> -h' for a command's flags.
`

// Run executes one command and returns the process exit code.
func Run(args []string, dir string, stdin io.Reader, stdout, stderr io.Writer) int {
	e := &env{dir: dir, stdin: stdin, stdout: stdout, stderr: stderr}
	if len(args) == 0 {
		e.out("%s", usage)
		return exitUsage
	}
	cmd, rest := args[0], args[1:]
	commands := map[string]func(*env, []string) int{
		"init": cmdInit, "review": cmdReview, "check": cmdCheck, "emit": cmdEmit,
		"explain": cmdExplain, "list": cmdList, "decisions": cmdDecisions,
		"lint": cmdLint, "sync": cmdSync, "prompt": cmdPrompt,
	}
	switch cmd {
	case "help", "-h", "--help":
		e.out("%s", usage)
		return exitOK
	case "version", "--version":
		e.out("opine %s\n", Version)
		return exitOK
	}
	run, ok := commands[cmd]
	if !ok {
		e.errf("opine: unknown command %q\n\n%s", cmd, usage)
		return exitUsage
	}
	return run(e, rest)
}

func (e *env) flags(name, synopsis string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(e.stderr)
	fs.Usage = func() {
		e.errf("Usage: opine %s\n", synopsis)
		fs.PrintDefaults()
	}
	return fs
}

// parse parses flags that may appear before or after positional arguments.
func parse(fs *flag.FlagSet, args []string) ([]string, bool) {
	var pos []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, false
		}
		args = fs.Args()
		if len(args) == 0 {
			return pos, true
		}
		pos = append(pos, args[0])
		args = args[1:]
	}
}

// load resolves the cascade and the structure file, printing any problems.
// ok is false when the configuration cannot be used.
func (e *env) load() (res *pack.Resolved, spec *structure.Spec, ok bool) {
	root, err := pack.FindRoot(e.dir)
	if err != nil {
		e.errf("opine: %v\n", err)
		return nil, nil, false
	}
	res, err = pack.Load(root)
	if err != nil {
		e.errf("opine: %v\n", err)
		return nil, nil, false
	}
	spec, probs, err := structure.Load(root)
	if err != nil {
		e.errf("opine: %v\n", err)
		return nil, nil, false
	}
	for _, p := range probs {
		res.Problems = append(res.Problems, pack.Problem{Where: pack.Dir + "/" + structure.File, Msg: p})
	}
	if len(res.Problems) > 0 {
		e.printProblems(res.Problems)
		return res, spec, false
	}
	return res, spec, true
}

func (e *env) printProblems(probs []pack.Problem) {
	last := ""
	for _, p := range probs {
		if p.Where != last {
			e.errf("%s\n", p.Where)
			last = p.Where
		}
		e.errf("  %s\n", p.Msg)
	}
	e.errf("\n%s in the configuration. Nothing was checked.\n", plural(len(probs), "problem"))
}

func plural(n int, word string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", word)
	}
	return fmt.Sprintf("%d %ss", n, word)
}

func cwd() string {
	d, err := os.Getwd()
	if err != nil {
		return "."
	}
	return d
}

// Main is the entry point used by cmd/opine.
func Main() {
	os.Exit(Run(os.Args[1:], cwd(), os.Stdin, os.Stdout, os.Stderr))
}

func pad(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len(s))
}
