// Package record parses and validates convention records: one markdown file
// per convention, YAML frontmatter for the machine, a short body for the human.
package record

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

type Level string

const (
	Must   Level = "must"
	Should Level = "should"
	Off    Level = "off"
)

type Status string

const (
	Proposed   Status = "proposed"
	Accepted   Status = "accepted"
	Rejected   Status = "rejected"
	Deprecated Status = "deprecated"
)

// Topics is the closed vocabulary a convention is filed under. The topic is
// the directory a record lives in, so it is never written twice.
var Topics = []string{
	"layout", "naming", "boundaries", "design", "errors", "testing", "api",
	"data", "dependencies", "style", "docs", "workflow", "security",
	"performance", "observability",
}

// Limits keep a record readable in one glance. They are part of the format.
const (
	MaxTitle        = 100
	MaxWhy          = 300
	MaxBodyLines    = 40
	MaxExampleLines = 15
	MaxExamples     = 2
)

type Example struct {
	Lang string
	Code string
}

type Evidence struct {
	Follows  int      `yaml:"follows"`
	Violates int      `yaml:"violates"`
	Examples []string `yaml:"examples,omitempty"`
}

// StringList accepts either a single string or a list in YAML.
type StringList []string

func (s *StringList) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind == yaml.ScalarNode {
		*s = StringList{n.Value}
		return nil
	}
	var l []string
	if err := n.Decode(&l); err != nil {
		return err
	}
	*s = l
	return nil
}

type Check struct {
	Kind    string     `yaml:"kind"`
	Pattern string     `yaml:"pattern,omitempty"`
	Unless  string     `yaml:"unless,omitempty"`
	Style   string     `yaml:"style,omitempty"`
	Target  string     `yaml:"target,omitempty"`
	Files   StringList `yaml:"files,omitempty"`
	Needs   StringList `yaml:"needs,omitempty"`
	Max     int        `yaml:"max,omitempty"`
	Cmd     string     `yaml:"cmd,omitempty"`
	Message string     `yaml:"message,omitempty"`
}

type frontmatter struct {
	Level    Level      `yaml:"level"`
	Status   Status     `yaml:"status"`
	Paths    StringList `yaml:"paths"`
	Exclude  StringList `yaml:"exclude"`
	Decides  string     `yaml:"decides"`
	Option   string     `yaml:"option"`
	Check    *Check     `yaml:"check"`
	Review   string     `yaml:"review"`
	Emit     *bool      `yaml:"emit"`
	See      StringList `yaml:"see"`
	Tags     StringList `yaml:"tags"`
	Evidence *Evidence  `yaml:"evidence"`
	Reason   string     `yaml:"reason"`
}

type Record struct {
	ID       string
	Topic    string
	Title    string
	Why      string
	Level    Level
	Status   Status
	Paths    []string
	Exclude  []string
	Decides  string
	Option   string
	Check    *Check
	Review   string
	Emit     bool
	See      []string
	Tags     []string
	Evidence *Evidence
	Reason   string
	Bad      []Example
	Good     []Example

	BodyLines int
	Source    string // file the record was read from, relative to its layer
	Layer     string // layer that defined it; set by the cascade
	Local     bool   // defined in the repo itself rather than a pack
}

// Mechanical reports whether a machine enforces the record.
func (r *Record) Mechanical() bool { return r.Check != nil }

var (
	labelRe = regexp.MustCompile(`(?i)^\**\s*(bad|good)\s*:?\s*\**\s*:?\s*$`)
	slugRe  = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
)

// SplitFrontmatter separates the YAML frontmatter from the markdown body.
func SplitFrontmatter(data []byte) (fm, body []byte, err error) {
	data = bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
	if !bytes.HasPrefix(data, []byte("---\n")) {
		return nil, nil, fmt.Errorf("missing frontmatter: file must start with ---")
	}
	rest := data[4:]
	if bytes.HasPrefix(rest, []byte("---\n")) {
		return nil, rest[4:], nil
	}
	end := bytes.Index(rest, []byte("\n---\n"))
	if end < 0 {
		if bytes.HasSuffix(rest, []byte("\n---")) {
			return rest[:len(rest)-4], nil, nil
		}
		return nil, nil, fmt.Errorf("unterminated frontmatter: missing closing ---")
	}
	return rest[:end+1], rest[end+5:], nil
}

// Parse reads one record. id is "<topic>/<slug>", derived from the file path.
func Parse(id, source string, data []byte) (*Record, error) {
	fmBytes, body, err := SplitFrontmatter(data)
	if err != nil {
		return nil, err
	}
	var fm frontmatter
	dec := yaml.NewDecoder(bytes.NewReader(fmBytes))
	dec.KnownFields(true)
	if err := dec.Decode(&fm); err != nil && err.Error() != "EOF" {
		return nil, fmt.Errorf("frontmatter: %w", err)
	}
	r := &Record{
		ID: id, Source: source,
		Level: fm.Level, Status: fm.Status,
		Paths: fm.Paths, Exclude: fm.Exclude,
		Decides: fm.Decides, Option: fm.Option,
		Check: fm.Check, Review: strings.TrimSpace(fm.Review),
		Emit: fm.Emit == nil || *fm.Emit,
		See:  fm.See, Tags: fm.Tags, Evidence: fm.Evidence,
		Reason: strings.TrimSpace(fm.Reason),
	}
	if i := strings.Index(id, "/"); i > 0 {
		r.Topic = id[:i]
	}
	if r.Level == "" {
		r.Level = Should
	}
	if r.Status == "" {
		r.Status = Accepted
	}
	parseBody(r, string(body))
	return r, nil
}

func parseBody(r *Record, body string) {
	lines := strings.Split(strings.Trim(body, "\n"), "\n")
	if len(lines) == 1 && lines[0] == "" {
		lines = nil
	}
	r.BodyLines = len(lines)
	var why []string
	label := ""
	seenTitle, inWhy := false, false
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			lang := strings.TrimSpace(strings.TrimPrefix(trimmed, "```"))
			var code []string
			for i++; i < len(lines) && !strings.HasPrefix(strings.TrimSpace(lines[i]), "```"); i++ {
				code = append(code, lines[i])
			}
			ex := Example{Lang: lang, Code: strings.Join(code, "\n")}
			switch label {
			case "bad":
				r.Bad = append(r.Bad, ex)
			case "good":
				r.Good = append(r.Good, ex)
			}
			label, inWhy = "", false
			continue
		}
		if m := labelRe.FindStringSubmatch(trimmed); m != nil {
			label, inWhy = strings.ToLower(m[1]), false
			continue
		}
		if !seenTitle {
			if strings.HasPrefix(trimmed, "# ") {
				r.Title = strings.TrimSpace(trimmed[2:])
				seenTitle, inWhy = true, true
			}
			continue
		}
		if inWhy && trimmed != "" {
			why = append(why, trimmed)
		}
	}
	r.Why = strings.Join(why, " ")
}

func isTopic(t string) bool {
	for _, x := range Topics {
		if x == t {
			return true
		}
	}
	return false
}

// Lint returns every way the record breaks the format. An empty result means
// the record is valid.
func Lint(r *Record) []string {
	var p []string
	add := func(f string, a ...any) { p = append(p, fmt.Sprintf(f, a...)) }

	topic, slug, ok := strings.Cut(r.ID, "/")
	switch {
	case !ok:
		add("id %q must be <topic>/<slug>", r.ID)
	case !isTopic(topic):
		add("unknown topic %q; topics are: %s", topic, strings.Join(Topics, ", "))
	case !slugRe.MatchString(slug):
		add("slug %q must be kebab-case", slug)
	}

	if r.Title == "" {
		add("missing title: the body must start with '# <the rule, as one sentence>'")
	} else if n := len([]rune(r.Title)); n > MaxTitle {
		add("title is %d characters; the limit is %d. State the rule in one short sentence", n, MaxTitle)
	}
	if n := len([]rune(r.Why)); n > MaxWhy {
		add("why is %d characters; the limit is %d. Link longer reasoning with 'see:'", n, MaxWhy)
	}
	if r.BodyLines > MaxBodyLines {
		add("body is %d lines; the limit is %d", r.BodyLines, MaxBodyLines)
	}
	for _, set := range []struct {
		name string
		ex   []Example
	}{{"bad", r.Bad}, {"good", r.Good}} {
		if len(set.ex) > MaxExamples {
			add("%d %s examples; the limit is %d", len(set.ex), set.name, MaxExamples)
		}
		for _, e := range set.ex {
			if n := strings.Count(e.Code, "\n") + 1; n > MaxExampleLines {
				add("a %s example is %d lines; the limit is %d", set.name, n, MaxExampleLines)
			}
		}
	}

	switch r.Level {
	case Must, Should, Off:
	default:
		add("level %q must be one of: must, should, off", r.Level)
	}
	switch r.Status {
	case Proposed, Accepted, Rejected, Deprecated:
	default:
		add("status %q must be one of: proposed, accepted, rejected, deprecated", r.Status)
	}
	if r.Status == Accepted && r.Level == Must && r.Why == "" {
		add("a 'must' convention needs a why: one or two sentences under the title")
	}
	if (r.Status == Rejected || r.Status == Deprecated) && r.Reason == "" {
		add("status %s needs a 'reason'", r.Status)
	}

	switch {
	case r.Check != nil && r.Review != "":
		add("set either 'check' or 'review', not both")
	case r.Check == nil && r.Review == "":
		add("no enforcement: add a 'check', or 'review: <how a reviewer spots a violation>'")
	case r.Check != nil:
		p = append(p, r.Check.validate(len(r.Paths) > 0)...)
	}

	if (r.Decides == "") != (r.Option == "") {
		add("'decides' and 'option' must be set together")
	}
	return p
}

var checkKinds = "forbid, require, exists, absent, name, pair, max-lines, run"

func (c *Check) validate(hasPaths bool) []string {
	var p []string
	add := func(f string, a ...any) { p = append(p, "check: "+fmt.Sprintf(f, a...)) }
	needPaths := func() {
		if !hasPaths {
			add("kind %s needs 'paths' on the record to say which files it covers", c.Kind)
		}
	}
	needPattern := func() {
		if c.Pattern == "" {
			add("kind %s needs a 'pattern'", c.Kind)
		} else if _, err := regexp.Compile(c.Pattern); err != nil {
			add("pattern does not compile (RE2 syntax, no lookaround): %v", err)
		}
	}
	if c.Unless != "" {
		if c.Kind != "forbid" {
			add("'unless' only applies to kind forbid")
		} else if _, err := regexp.Compile(c.Unless); err != nil {
			add("unless does not compile (RE2 syntax, no lookaround): %v", err)
		}
	}
	switch c.Kind {
	case "forbid", "require":
		needPaths()
		needPattern()
	case "exists", "absent":
		if len(c.Files) == 0 {
			add("kind %s needs 'files'", c.Kind)
		}
	case "name":
		needPaths()
		switch c.Style {
		case "kebab-case", "snake_case", "camelCase", "PascalCase":
		case "regex":
			needPattern()
		default:
			add("style %q must be one of: kebab-case, snake_case, camelCase, PascalCase, regex", c.Style)
		}
		if c.Target != "" && c.Target != "file" && c.Target != "dir" {
			add("target %q must be file or dir", c.Target)
		}
	case "pair":
		needPaths()
		if len(c.Needs) == 0 {
			add("kind pair needs 'needs', e.g. \"{dir}/{name}.test{ext}\"")
		}
	case "max-lines":
		needPaths()
		if c.Max <= 0 {
			add("kind max-lines needs a positive 'max'")
		}
	case "run":
		if strings.TrimSpace(c.Cmd) == "" {
			add("kind run needs a 'cmd'")
		}
	case "":
		add("missing 'kind'; kinds are: %s", checkKinds)
	default:
		add("unknown kind %q; kinds are: %s", c.Kind, checkKinds)
	}
	return p
}

// SetFields rewrites frontmatter keys in place, preserving the body and the
// order of existing keys. An empty value removes the key.
func SetFields(data []byte, set map[string]string, order []string) ([]byte, error) {
	fm, body, err := SplitFrontmatter(data)
	if err != nil {
		return nil, err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(fm, &doc); err != nil {
		return nil, err
	}
	if len(doc.Content) == 0 {
		doc = yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{Kind: yaml.MappingNode}}}
	}
	m := doc.Content[0]
	for _, k := range order {
		v, ok := set[k]
		if !ok {
			continue
		}
		found := false
		for i := 0; i+1 < len(m.Content); i += 2 {
			if m.Content[i].Value != k {
				continue
			}
			found = true
			if v == "" {
				m.Content = append(m.Content[:i], m.Content[i+2:]...)
			} else {
				m.Content[i+1] = &yaml.Node{Kind: yaml.ScalarNode, Value: v}
			}
			break
		}
		if !found && v != "" {
			m.Content = append(m.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Value: k},
				&yaml.Node{Kind: yaml.ScalarNode, Value: v})
		}
	}
	var out bytes.Buffer
	enc := yaml.NewEncoder(&out)
	enc.SetIndent(2)
	if err := enc.Encode(m); err != nil {
		return nil, err
	}
	enc.Close()
	return []byte("---\n" + out.String() + "---\n" + string(body)), nil
}
