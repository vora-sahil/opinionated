package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"github.com/vora-sahil/opinionated/internal/check"
	"github.com/vora-sahil/opinionated/internal/pack"
	"github.com/vora-sahil/opinionated/internal/record"
)

func cmdCheck(e *env, args []string) int {
	fs := e.flags("check", "check [flags] [file ...]")
	format := fs.String("format", "text", "output format: text, json or sarif")
	update := fs.Bool("update-baseline", false, "record today's violations as the accepted starting point")
	noBaseline := fs.Bool("no-baseline", false, "report violations the baseline covers too")
	hook := fs.Bool("hook", false, "run as an agent hook: read the edited file from stdin JSON, report to stderr, exit 2 on findings")
	only, ok := parse(fs, args)
	if !ok {
		return exitUsage
	}
	if *format != "text" && *format != "json" && *format != "sarif" {
		e.errf("opine: unknown format %q\n", *format)
		return exitUsage
	}

	var hookFile string
	if *hook {
		var in struct {
			Cwd       string `json:"cwd"`
			ToolInput struct {
				FilePath string `json:"file_path"`
			} `json:"tool_input"`
		}
		data, _ := io.ReadAll(e.stdin)
		if json.Unmarshal(data, &in) != nil || in.ToolInput.FilePath == "" {
			return exitOK
		}
		hookFile = in.ToolInput.FilePath
		if !filepath.IsAbs(hookFile) {
			hookFile = filepath.Join(e.dir, hookFile)
		}
	}

	res, spec, ok := e.load()
	if !ok {
		if *hook {
			return exitViolations // shown to the user, not fed to the agent as a finding
		}
		return exitUsage
	}

	for i, f := range only {
		if !filepath.IsAbs(f) {
			f = filepath.Join(e.dir, f)
		}
		only[i] = f
	}
	if hookFile != "" {
		only = []string{hookFile}
	}
	for i, f := range only {
		rel, err := filepath.Rel(res.Root, f)
		if err != nil || strings.HasPrefix(rel, "..") {
			if *hook {
				return exitOK
			}
			e.errf("opine: %s is outside the repository\n", f)
			return exitUsage
		}
		only[i] = filepath.ToSlash(rel)
	}
	partial := len(only) > 0

	files, err := check.Files(res)
	if err != nil {
		e.errf("opine: %v\n", err)
		return exitUsage
	}
	src := check.NewSource(res.Root)
	found := check.Run(res, files, src, check.Options{Only: only})
	if spec != nil {
		lay, shape := spec.Check(files)
		for _, v := range shape {
			if !partial || touches(only, v.File) {
				found = append(found, v)
			}
		}
		found = append(found, lay.CheckImports(src, files, only)...)
	}
	found = check.Suppress(src, found)

	if *update {
		if partial {
			e.errf("opine: --update-baseline works on the whole repository; drop the file arguments\n")
			return exitUsage
		}
		if err := check.WriteBaseline(res.Root, found); err != nil {
			e.errf("opine: %v\n", err)
			return exitUsage
		}
		e.out("Baseline records %s. From here on, no file may exceed its count.\n", plural(len(found), "violation"))
		return exitOK
	}

	covered, shrinkable := 0, 0
	if !*noBaseline {
		b, err := check.LoadBaseline(res.Root)
		if err != nil {
			e.errf("opine: %s: %v\n", check.BaselineFile, err)
			return exitUsage
		}
		found, covered, shrinkable = b.Filter(found)
		if partial {
			shrinkable = 0
		}
	}

	must := 0
	for _, v := range found {
		if v.Level == record.Must {
			must++
		}
	}

	if *hook {
		if len(found) == 0 {
			return exitOK
		}
		e.errf("opine: this edit breaks %s. Fix them, or run 'opine explain <id>' for the reason and an example.\n", plural(len(found), "convention"))
		printText(e.stderr, found, res)
		return 2
	}

	switch *format {
	case "json":
		enc := json.NewEncoder(e.stdout)
		enc.SetIndent("", "  ")
		if found == nil {
			found = []check.Violation{}
		}
		_ = enc.Encode(map[string]any{"violations": found, "baselined": covered})
	case "sarif":
		writeSarif(e.stdout, found, res)
	default:
		printText(e.stdout, found, res)
		e.out("%s", summary(found, must, covered, shrinkable, reviewCount(res)))
	}
	if must > 0 {
		return exitViolations
	}
	return exitOK
}

func touches(only []string, file string) bool {
	for _, o := range only {
		if o == file || strings.HasPrefix(o, file+"/") {
			return true
		}
	}
	return false
}

func reviewCount(res *pack.Resolved) int {
	n := 0
	for _, r := range res.Active() {
		if !r.Mechanical() {
			n++
		}
	}
	return n
}

func printText(w io.Writer, found []check.Violation, res *pack.Resolved) {
	last := "\x00"
	for _, v := range found {
		if v.File != last {
			if last != "\x00" {
				fmt.Fprintln(w)
			}
			name := v.File
			if name == "" {
				name = "(repository)"
			}
			fmt.Fprintln(w, name)
			last = v.File
		}
		line := "    "
		if v.Line > 0 {
			line = fmt.Sprintf("%4d", v.Line)
		}
		fmt.Fprintf(w, "  %s  %s  %s  %s\n", line, pad(string(v.Level), 6), pad(v.ID, 34), v.Message)
	}
	if len(found) > 0 {
		fmt.Fprintln(w)
	}
}

func summary(found []check.Violation, must, covered, shrinkable, review int) string {
	var b strings.Builder
	if len(found) == 0 {
		b.WriteString("No violations.")
	} else {
		fmt.Fprintf(&b, "%s: %d must, %d should.", plural(len(found), "violation"), must, len(found)-must)
		if must == 0 {
			b.WriteString(" Only 'must' fails the check.")
		}
	}
	if covered > 0 {
		fmt.Fprintf(&b, " %d more covered by the baseline.", covered)
	}
	b.WriteString("\n")
	if shrinkable > 0 {
		fmt.Fprintf(&b, "%s improved since the baseline was taken; run 'opine check --update-baseline' to lock that in.\n", plural(shrinkable, "file"))
	}
	if review > 0 {
		fmt.Fprintf(&b, "%s enforced in review, not by this command; 'opine list' shows them.\n", plural(review, "convention"))
	}
	if len(found) > 0 {
		b.WriteString("'opine explain <id>' shows the reason and an example.\n")
	}
	return b.String()
}

func writeSarif(w io.Writer, found []check.Violation, res *pack.Resolved) {
	type msg struct {
		Text string `json:"text"`
	}
	type rule struct {
		ID    string `json:"id"`
		Short msg    `json:"shortDescription"`
	}
	ruleSet := map[string]rule{}
	results := []map[string]any{}
	for _, v := range found {
		title := v.Message
		if r := res.Find(v.ID); r != nil {
			title = r.Title
		}
		ruleSet[v.ID] = rule{ID: v.ID, Short: msg{title}}
		level := "warning"
		if v.Level == record.Must {
			level = "error"
		}
		result := map[string]any{"ruleId": v.ID, "level": level, "message": msg{v.Message}}
		if v.File != "" {
			loc := map[string]any{"artifactLocation": map[string]string{"uri": v.File}}
			if v.Line > 0 {
				loc["region"] = map[string]int{"startLine": v.Line}
			}
			result["locations"] = []map[string]any{{"physicalLocation": loc}}
		}
		results = append(results, result)
	}
	rules := make([]rule, 0, len(ruleSet))
	for _, r := range ruleSet {
		rules = append(rules, r)
	}
	sort.Slice(rules, func(i, j int) bool { return rules[i].ID < rules[j].ID })
	doc := map[string]any{
		"$schema": "https://json.schemastore.org/sarif-2.1.0.json",
		"version": "2.1.0",
		"runs": []map[string]any{{
			"tool": map[string]any{"driver": map[string]any{
				"name": "opine", "version": Version,
				"informationUri": "https://github.com/vora-sahil/opinionated",
				"rules":          rules,
			}},
			"results": results,
		}},
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(doc)
}
