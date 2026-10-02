package record

import (
	"strings"
	"testing"
)

const sample = "---\nlevel: must\npaths: [\"src/**/*.kt\"]\ncheck:\n  kind: forbid\n  pattern: 'catch \\([^)]*\\)\\s*\\{\\s*\\}'\n---\n# Never swallow exceptions\n\nSilent failures hid the billing outage\nfor six hours.\n\nBad:\n```kotlin\ntry { charge() } catch (e: Exception) {}\n```\n\n**Good:**\n```kotlin\ntry { charge() } catch (e: Exception) { throw e }\n```\n"

func TestParse(t *testing.T) {
	r, err := Parse("errors/no-swallowed-exceptions", "x.md", []byte(sample))
	if err != nil {
		t.Fatal(err)
	}
	if r.Title != "Never swallow exceptions" {
		t.Errorf("title = %q", r.Title)
	}
	if r.Why != "Silent failures hid the billing outage for six hours." {
		t.Errorf("why = %q", r.Why)
	}
	if r.Level != Must || r.Status != Accepted || r.Topic != "errors" || !r.Emit {
		t.Errorf("defaults wrong: %+v", r)
	}
	if len(r.Bad) != 1 || len(r.Good) != 1 || r.Bad[0].Lang != "kotlin" {
		t.Fatalf("examples: bad=%v good=%v", r.Bad, r.Good)
	}
	if !strings.Contains(r.Good[0].Code, "throw e") {
		t.Errorf("good code = %q", r.Good[0].Code)
	}
	if p := Lint(r); len(p) != 0 {
		t.Errorf("unexpected problems: %v", p)
	}
}

func TestParseRejectsUnknownField(t *testing.T) {
	_, err := Parse("errors/x", "x.md", []byte("---\nseverity: high\n---\n# T\n"))
	if err == nil || !strings.Contains(err.Error(), "severity") {
		t.Fatalf("want unknown-field error, got %v", err)
	}
}

func TestLint(t *testing.T) {
	cases := []struct {
		name, id, doc, want string
	}{
		{"bad topic", "vibes/x", "---\nreview: look\n---\n# T\n", "unknown topic"},
		{"bad slug", "errors/No_Good", "---\nreview: look\n---\n# T\n", "kebab-case"},
		{"no title", "errors/x", "---\nreview: look\n---\nhello\n", "missing title"},
		{"long title", "errors/x", "---\nreview: look\n---\n# " + strings.Repeat("a", 101) + "\n", "title is 101"},
		{"long why", "errors/x", "---\nreview: look\n---\n# T\n\n" + strings.Repeat("a", 301) + "\n", "why is 301"},
		{"must needs why", "errors/x", "---\nlevel: must\nreview: look\n---\n# T\n", "needs a why"},
		{"no enforcement", "errors/x", "---\nlevel: should\n---\n# T\n", "no enforcement"},
		{"both enforcement", "errors/x", "---\nreview: look\ncheck: {kind: run, cmd: 'true'}\n---\n# T\n", "not both"},
		{"bad level", "errors/x", "---\nlevel: critical\nreview: look\n---\n# T\n", "level"},
		{"rejected needs reason", "errors/x", "---\nstatus: rejected\nreview: look\n---\n# T\n", "needs a 'reason'"},
		{"decides without option", "errors/x", "---\ndecides: errors/propagation\nreview: look\n---\n# T\n", "set together"},
		{"forbid needs paths", "errors/x", "---\ncheck: {kind: forbid, pattern: x}\n---\n# T\n", "needs 'paths'"},
		{"bad regex", "errors/x", "---\npaths: ['**']\ncheck: {kind: forbid, pattern: '(?!x)'}\n---\n# T\n", "does not compile"},
		{"bad kind", "errors/x", "---\ncheck: {kind: grep}\n---\n# T\n", "unknown kind"},
		{"name style", "naming/x", "---\npaths: ['**']\ncheck: {kind: name, style: Title}\n---\n# T\n", "style"},
		{"long example", "errors/x", "---\nreview: look\n---\n# T\n\nBad:\n```\n" + strings.Repeat("x\n", 16) + "```\n", "example is 16 lines"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r, err := Parse(c.id, "x.md", []byte(c.doc))
			if err != nil {
				t.Fatal(err)
			}
			got := strings.Join(Lint(r), "\n")
			if !strings.Contains(got, c.want) {
				t.Errorf("want problem containing %q, got:\n%s", c.want, got)
			}
		})
	}
}

func TestSetFields(t *testing.T) {
	out, err := SetFields([]byte("---\nstatus: proposed\nlevel: should\nreview: look\n---\n# T\n\nbody\n"),
		map[string]string{"status": "rejected", "reason": "not how we work"}, []string{"status", "reason"})
	if err != nil {
		t.Fatal(err)
	}
	r, err := Parse("errors/x", "x.md", out)
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != Rejected || r.Reason != "not how we work" || r.Title != "T" || r.Review != "look" {
		t.Errorf("got %+v\n%s", r, out)
	}
	if !strings.HasSuffix(string(out), "# T\n\nbody\n") {
		t.Errorf("body not preserved:\n%s", out)
	}
}
