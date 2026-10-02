package cli

import (
	"embed"
	"strings"
	"text/template"

	"github.com/vora-sahil/opinionated/internal/fsx"
	"github.com/vora-sahil/opinionated/internal/infer"
	"github.com/vora-sahil/opinionated/internal/record"
)

//go:embed prompts/*.md
var promptFS embed.FS

// cmdPrompt prints instructions for a coding agent. opine makes no model
// calls of its own: the judgment is done by whichever agent the user already
// has, and opine supplies the format, the validation and the human gate.
func cmdPrompt(e *env, args []string) int {
	fs := e.flags("prompt", "prompt <mine|review>")
	pos, ok := parse(fs, args)
	if !ok || len(pos) != 1 || (pos[0] != "mine" && pos[0] != "review") {
		fs.Usage()
		e.errf("\n  mine    instructions for an agent to propose conventions from the existing code\n  review  instructions for an agent to review a change against review-enforced conventions\n\nGive the output to your coding agent as its instructions.\n")
		return exitUsage
	}
	res, _, ok := e.load()
	if !ok {
		return exitUsage
	}
	tmpl, err := template.ParseFS(promptFS, "prompts/"+pos[0]+".md")
	if err != nil {
		e.errf("opine: %v\n", err)
		return exitUsage
	}

	type decision struct{ ID, Question, Options string }
	type existing struct{ ID, State, Title string }
	type conv struct{ ID, Title, Why, Review, Level, Paths string }
	data := struct {
		Topics                       []string
		Linters                      string
		Undecided                    []decision
		Existing                     []existing
		Conventions                  []conv
		MaxTitle, MaxWhy, MaxExample int
	}{Topics: record.Topics, MaxTitle: record.MaxTitle, MaxWhy: record.MaxWhy, MaxExample: record.MaxExampleLines}

	if files, err := fsx.List(res.Root); err == nil {
		data.Linters = strings.Join(infer.Analyze(res.Root, files).Linters, ", ")
	}
	for _, d := range res.Decisions {
		if _, ok := res.Config.Decisions[d.ID]; !ok {
			data.Undecided = append(data.Undecided, decision{d.ID, d.Question, strings.Join(d.Options, ", ")})
		}
	}
	for _, r := range res.Records {
		active, why := res.State(r)
		state := "in force"
		if !active {
			state = why
		}
		if r.Decides != "" && r.Status == record.Accepted && !active {
			continue // an option nobody chose is not a recorded convention
		}
		data.Existing = append(data.Existing, existing{r.ID, state, r.Title})
		if active && !r.Mechanical() {
			data.Conventions = append(data.Conventions, conv{r.ID, r.Title, r.Why, r.Review, string(r.Level), strings.Join(r.Paths, ", ")})
		}
	}
	if err := tmpl.Execute(e.stdout, data); err != nil {
		e.errf("opine: %v\n", err)
		return exitUsage
	}
	return exitOK
}
