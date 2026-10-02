package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/vora-sahil/opinionated/internal/pack"
	"github.com/vora-sahil/opinionated/internal/record"
)

func cmdReview(e *env, args []string) int {
	fs := e.flags("review", "review [--accept <id> [--why <text>] | --reject <id> --reason <text>]")
	accept := fs.String("accept", "", "accept this proposed convention without prompting")
	reject := fs.String("reject", "", "reject this proposed convention without prompting")
	why := fs.String("why", "", "with --accept: the reason for the convention")
	reason := fs.String("reason", "", "with --reject: why it does not apply here")
	if _, ok := parse(fs, args); !ok {
		return exitUsage
	}
	root, err := pack.FindRoot(e.dir)
	if err != nil {
		e.errf("opine: %v\n", err)
		return exitUsage
	}
	res, err := pack.Load(root)
	if err != nil {
		e.errf("opine: %v\n", err)
		return exitUsage
	}
	var proposed []*record.Record
	for _, r := range res.Records {
		if r.Status == record.Proposed && r.Local {
			proposed = append(proposed, r)
		}
	}

	if id := *accept + *reject; id != "" {
		for _, r := range proposed {
			if r.ID != id {
				continue
			}
			if *accept != "" {
				err = acceptRecord(root, r, *why)
			} else {
				err = rejectRecord(root, r, *reason)
			}
			if err != nil {
				e.errf("opine: %v\n", err)
				return exitUsage
			}
			e.out("%s: done.\n", id)
			return exitOK
		}
		e.errf("opine: %q is not a proposed convention in this repository\n", id)
		return exitUsage
	}

	if len(proposed) == 0 {
		e.out("Nothing is waiting for review.\n")
		return exitOK
	}
	in := bufio.NewReader(e.stdin)
	ask := func(prompt string) (string, bool) {
		e.out("%s", prompt)
		line, err := in.ReadString('\n')
		if err != nil && line == "" {
			return "", false
		}
		return strings.TrimSpace(line), true
	}
	accepted, rejected := 0, 0
loop:
	for i, r := range proposed {
		e.out("\n[%d/%d] %s  (%s)\n\n  %s\n", i+1, len(proposed), r.ID, r.Level, r.Title)
		if r.Why != "" {
			e.out("\n  %s\n", r.Why)
		}
		e.out("\n")
		if ev := r.Evidence; ev != nil {
			e.out("  %s %d of %d places already do this\n", pad("Evidence", 9), ev.Follows, ev.Follows+ev.Violates)
			for _, x := range ev.Examples {
				e.out("  %s not yet: %s\n", pad("", 9), x)
			}
		}
		if len(r.Paths) > 0 {
			e.out("  %s %s\n", pad("Applies", 9), strings.Join(r.Paths, ", "))
		}
		if r.Check != nil {
			e.out("  %s %s\n", pad("Check", 9), describeCheck(r.Check))
		} else {
			e.out("  %s %s\n", pad("Review", 9), r.Review)
		}
		for {
			ans, ok := ask("\n  Is this how you do it here?  [y]es  [n]o  [s]kip  [q]uit > ")
			if !ok {
				break loop
			}
			switch strings.ToLower(ans) {
			case "y", "yes", "a":
				w := ""
				if r.Level == record.Must && r.Why == "" {
					if w, ok = ask("  Why? One or two sentences, so the next person does not have to guess > "); !ok || w == "" {
						e.out("  A 'must' needs a reason. Skipped.\n")
						continue loop
					}
				}
				if err := acceptRecord(root, r, w); err != nil {
					e.errf("opine: %v\n", err)
					return exitUsage
				}
				accepted++
			case "n", "no", "r":
				because, _ := ask("  Why not? Recorded so it is not proposed again > ")
				if because == "" {
					because = "not how we work here"
				}
				if err := rejectRecord(root, r, because); err != nil {
					e.errf("opine: %v\n", err)
					return exitUsage
				}
				rejected++
			case "s", "skip", "":
			case "q", "quit":
				break loop
			default:
				continue
			}
			break
		}
	}
	e.out("\nAccepted %d, rejected %d, %d still proposed.\n", accepted, rejected, len(proposed)-accepted-rejected)
	if accepted > 0 {
		e.out("Next: 'opine check' to see where the code stands ('--update-baseline' to grandfather it), then 'opine emit'.\n")
	}
	return exitOK
}

func recordPath(root string, r *record.Record) string {
	return filepath.Join(root, filepath.FromSlash(r.Source))
}

// acceptRecord puts a proposed convention in force. The evidence that
// justified proposing it is dropped: from here on the check is the evidence.
func acceptRecord(root string, r *record.Record, why string) error {
	if r.Level == record.Must && r.Why == "" && strings.TrimSpace(why) == "" {
		return fmt.Errorf("%s is a 'must' and needs a reason; pass --why", r.ID)
	}
	p := recordPath(root, r)
	data, err := os.ReadFile(p)
	if err != nil {
		return err
	}
	out, err := record.SetFields(data, map[string]string{"status": "", "evidence": ""}, []string{"status", "evidence"})
	if err != nil {
		return err
	}
	if why = strings.TrimSpace(why); why != "" && r.Why == "" {
		out = insertWhy(out, why)
	}
	return os.WriteFile(p, out, 0o644)
}

// rejectRecord keeps the file with its reason, so the same convention is not
// proposed again and the next person can see it was considered.
func rejectRecord(root string, r *record.Record, reason string) error {
	if strings.TrimSpace(reason) == "" {
		return fmt.Errorf("rejecting %s needs --reason", r.ID)
	}
	p := recordPath(root, r)
	data, err := os.ReadFile(p)
	if err != nil {
		return err
	}
	out, err := record.SetFields(data, map[string]string{"status": "rejected", "reason": reason}, []string{"status", "reason"})
	if err != nil {
		return err
	}
	return os.WriteFile(p, out, 0o644)
}

func insertWhy(data []byte, why string) []byte {
	lines := strings.Split(string(data), "\n")
	fences := 0
	for i, l := range lines {
		if l == "---" {
			fences++
			continue
		}
		if fences >= 2 && strings.HasPrefix(l, "# ") {
			rest := append([]string{"", why}, lines[i+1:]...)
			return []byte(strings.Join(append(lines[:i+1], rest...), "\n"))
		}
	}
	return data
}
