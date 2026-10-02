// Package pack loads a repository's opine configuration, the packs it
// extends, and resolves them into one set of conventions.
package pack

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/vora-sahil/opinionated/internal/record"
)

const (
	Dir        = ".opine"
	ConfigFile = "opine.yaml"
	LockFile   = "opine.lock"
	CacheDir   = "cache"
)

// Extend names one layer below the repository's own conventions.
type Extend struct {
	Builtin string `yaml:"builtin,omitempty"`
	Path    string `yaml:"path,omitempty"`
	Git     string `yaml:"git,omitempty"`
	Ref     string `yaml:"ref,omitempty"`
}

func (e *Extend) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind == yaml.ScalarNode {
		const p = "builtin:"
		if len(n.Value) > len(p) && n.Value[:len(p)] == p {
			e.Builtin = n.Value[len(p):]
			return nil
		}
		return fmt.Errorf("line %d: extends entry %q must be builtin:<name>, or a mapping with path: or git:", n.Line, n.Value)
	}
	type plain Extend
	return n.Decode((*plain)(e))
}

func (e Extend) Name() string {
	switch {
	case e.Builtin != "":
		return "builtin:" + e.Builtin
	case e.Path != "":
		return e.Path
	default:
		return e.Git
	}
}

// Override changes how an inherited convention applies here. It always
// carries a reason, so a deviation is a recorded decision rather than drift.
type Override struct {
	Level   record.Level      `yaml:"level,omitempty"`
	Paths   record.StringList `yaml:"paths,omitempty"`
	Exclude record.StringList `yaml:"exclude,omitempty"`
	Reason  string            `yaml:"reason"`
}

type EmitConfig struct {
	Targets []string `yaml:"targets,omitempty"`
	Budget  int      `yaml:"budget,omitempty"`
}

type Config struct {
	Version   int                 `yaml:"version"`
	Extends   []Extend            `yaml:"extends"`
	Decisions map[string]string   `yaml:"decisions"`
	Overrides map[string]Override `yaml:"overrides"`
	Ignore    []string            `yaml:"ignore"`
	Emit      EmitConfig          `yaml:"emit"`
}

// FindRoot walks up from dir to the directory holding .opine/opine.yaml.
func FindRoot(dir string) (string, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	for d := dir; ; d = filepath.Dir(d) {
		if _, err := os.Stat(filepath.Join(d, Dir, ConfigFile)); err == nil {
			return d, nil
		}
		if d == filepath.Dir(d) {
			return "", fmt.Errorf("no %s/%s found here or in any parent directory; run 'opine init'", Dir, ConfigFile)
		}
	}
}

func readConfig(root string) (*Config, error) {
	p := filepath.Join(root, Dir, ConfigFile)
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	var c Config
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&c); err != nil && err.Error() != "EOF" {
		return nil, fmt.Errorf("%s/%s: %w", Dir, ConfigFile, err)
	}
	if c.Version != 1 {
		return nil, fmt.Errorf("%s/%s: version must be 1", Dir, ConfigFile)
	}
	return &c, nil
}
