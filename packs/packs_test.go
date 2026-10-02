package packs_test

import (
	"testing"

	"github.com/vora-sahil/opinionated/internal/pack"
	"github.com/vora-sahil/opinionated/internal/testrepo"
)

// The pack that ships in the binary must be valid, every decision must have
// at least two real options or be a deliberate opt-in, and every option must
// be selectable.
func TestUniversalPackIsValid(t *testing.T) {
	root := testrepo.New(t, map[string]string{
		".opine/opine.yaml": "version: 1\nextends:\n  - builtin:universal\n",
	})
	res, err := pack.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range res.Problems {
		t.Errorf("%s", p)
	}
	if len(res.Decisions) == 0 || len(res.Active()) == 0 {
		t.Fatalf("pack looks empty: %d decisions, %d active", len(res.Decisions), len(res.Active()))
	}
	for _, d := range res.Decisions {
		if len(d.Options) == 0 {
			t.Errorf("decision %s has no options", d.ID)
		}
	}
	for _, r := range res.Records {
		if r.Why == "" {
			t.Errorf("%s: every shipped convention states its why", r.ID)
		}
		if r.Decides == "" {
			if ok, why := res.State(r); !ok {
				t.Errorf("%s should be in force by default: %s", r.ID, why)
			}
		}
	}
}
