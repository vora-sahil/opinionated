package pack_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vora-sahil/opinionated/internal/pack"
	"github.com/vora-sahil/opinionated/internal/testrepo"
)

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@t", "-c", "commit.gpgsign=false"}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// A git pack is pinned: a second machine resolves the same commit even after
// the branch has moved, until someone asks for an update.
func TestGitPackIsPinned(t *testing.T) {
	base := testrepo.New(t, map[string]string{
		"house/pack.yaml":                "name: house\n",
		"house/conventions/style/one.md": "---\nreview: look\n---\n# One\n",
	})
	house := filepath.Join(base, "house")
	git(t, house, "init", "-q", "-b", "main")
	git(t, house, "add", ".")
	git(t, house, "commit", "-q", "-m", "one")
	first := git(t, house, "rev-parse", "HEAD")

	repo := filepath.Join(base, "repo")
	testrepo.Write(t, repo, map[string]string{
		".opine/opine.yaml": "version: 1\nextends:\n  - git: " + house + "\n    ref: main\n",
	})
	res, err := pack.Load(repo)
	if err != nil || len(res.Problems) != 1 || !strings.Contains(res.Problems[0].Msg, "opine sync") {
		t.Fatalf("before sync: %v %v", err, res.Problems)
	}
	quiet := func(string, ...any) {}
	if err := pack.Sync(repo, false, quiet); err != nil {
		t.Fatal(err)
	}
	res, _ = pack.Load(repo)
	if len(res.Problems) != 0 || res.Find("style/one") == nil {
		t.Fatalf("after sync: %v", res.Problems)
	}

	testrepo.Write(t, house, map[string]string{"conventions/style/two.md": "---\nreview: look\n---\n# Two\n"})
	git(t, house, "add", ".")
	git(t, house, "commit", "-q", "-m", "two")

	if err := os.RemoveAll(filepath.Join(repo, ".opine", "cache")); err != nil {
		t.Fatal(err)
	}
	if err := pack.Sync(repo, false, quiet); err != nil {
		t.Fatal(err)
	}
	lock, _ := os.ReadFile(filepath.Join(repo, ".opine", "opine.lock"))
	if res, _ = pack.Load(repo); res.Find("style/two") != nil || !strings.Contains(string(lock), first) {
		t.Fatalf("a fresh checkout must honour the pinned commit %s:\n%s", first, lock)
	}
	if err := pack.Sync(repo, true, quiet); err != nil {
		t.Fatal(err)
	}
	if res, _ = pack.Load(repo); res.Find("style/two") == nil || len(res.Problems) != 0 {
		t.Fatalf("--update should move to the new commit: %v", res.Problems)
	}
}
