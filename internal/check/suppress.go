package check

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/vora-sahil/opinionated/internal/record"
)

// SuppressionID is reported when a suppression gives no reason.
const SuppressionID = "opine/suppression"

var ignoreRe = regexp.MustCompile(`opine-ignore(-file)?\s+([a-z0-9]+/[a-z0-9-]+)\s*:?\s*(.*)`)

const fileDirectiveLines = 10

type directive struct {
	id     string
	reason string
	line   int
	file   bool
}

func directives(lines []string) []directive {
	var out []directive
	for i, l := range lines {
		m := ignoreRe.FindStringSubmatch(l)
		if m == nil {
			continue
		}
		reason := strings.TrimSpace(strings.TrimRight(strings.TrimSpace(m[3]), "*/->#"))
		d := directive{id: m[2], reason: reason, line: i + 1, file: m[1] != ""}
		if d.file && i >= fileDirectiveLines {
			continue
		}
		out = append(out, d)
	}
	return out
}

// Suppress drops violations that the code explicitly opts out of with an
// 'opine-ignore <id>: <reason>' comment on the same or the previous line, or
// 'opine-ignore-file <id>: <reason>' in the first lines of a file. A
// suppression without a reason is itself a violation.
func Suppress(src *Source, in []Violation) []Violation {
	byFile := map[string][]directive{}
	var out []Violation
	reported := map[string]bool{}
	for _, v := range in {
		if v.File == "" {
			out = append(out, v)
			continue
		}
		ds, ok := byFile[v.File]
		if !ok {
			ds = directives(src.Lines(v.File))
			byFile[v.File] = ds
		}
		suppressed := false
		for _, d := range ds {
			if d.id != v.ID {
				continue
			}
			if !d.file && (v.Line == 0 || (d.line != v.Line && d.line != v.Line-1)) {
				continue
			}
			suppressed = true
			key := fmt.Sprintf("%s:%s:%d", v.File, d.id, d.line)
			if d.reason == "" && !reported[key] {
				reported[key] = true
				out = append(out, Violation{
					ID: SuppressionID, Level: record.Must, File: v.File, Line: d.line,
					Message: "A suppression needs a reason: opine-ignore " + d.id + ": <why this case is different>",
				})
			}
			break
		}
		if !suppressed {
			out = append(out, v)
		}
	}
	Sort(out)
	return out
}
