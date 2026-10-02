package check

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/vora-sahil/opinionated/internal/pack"
)

const BaselineFile = "baseline.json"

// Baseline records how many violations of each convention each file had when
// the convention was adopted. Counts, not line numbers, so edits elsewhere in
// a file do not disturb it. A file may never exceed its count.
type Baseline struct {
	Version int                       `json:"version"`
	Counts  map[string]map[string]int `json:"counts"`
}

func baselinePath(root string) string { return filepath.Join(root, pack.Dir, BaselineFile) }

func LoadBaseline(root string) (*Baseline, error) {
	b := &Baseline{Version: 1, Counts: map[string]map[string]int{}}
	data, err := os.ReadFile(baselinePath(root))
	if os.IsNotExist(err) {
		return b, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, b); err != nil {
		return nil, err
	}
	if b.Counts == nil {
		b.Counts = map[string]map[string]int{}
	}
	return b, nil
}

func count(v []Violation) map[string]map[string]int {
	c := map[string]map[string]int{}
	for _, x := range v {
		if c[x.ID] == nil {
			c[x.ID] = map[string]int{}
		}
		c[x.ID][x.File]++
	}
	return c
}

// Filter removes violations the baseline covers. A file over its count
// reports all of that convention's violations in it, since counts cannot say
// which one is new. shrinkable counts entries that could now be tightened.
func (b *Baseline) Filter(in []Violation) (out []Violation, covered, shrinkable int) {
	now := count(in)
	for _, v := range in {
		if now[v.ID][v.File] <= b.Counts[v.ID][v.File] {
			covered++
			continue
		}
		out = append(out, v)
	}
	for id, files := range b.Counts {
		for f, n := range files {
			if now[id][f] < n {
				shrinkable++
			}
		}
	}
	return out, covered, shrinkable
}

// WriteBaseline records the given violations as the accepted starting point.
func WriteBaseline(root string, v []Violation) error {
	b := Baseline{Version: 1, Counts: count(v)}
	if len(v) == 0 {
		err := os.Remove(baselinePath(root))
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(baselinePath(root), append(data, '\n'), 0o644)
}
