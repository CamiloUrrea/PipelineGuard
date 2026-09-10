// Package config loads PipelineGuard's optional .pipelineguard.yml file into a
// typed Config, falling back to built-in defaults for every field the file
// leaves unset. It only understands YAML: there is no generic multi-format
// loader here on purpose.
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"pipelineguard/internal/scoring"
)

// ScannersConfig toggles which security scanners PipelineGuard runs.
type ScannersConfig struct {
	Gitleaks bool `yaml:"gitleaks"`
	Trivy    bool `yaml:"trivy"`
	Semgrep  bool `yaml:"semgrep"`
}

// Config is the parsed representation of .pipelineguard.yml. See docs/config.md
// for the field-by-field schema.
type Config struct {
	Scanners        ScannersConfig `yaml:"scanners"`
	Enforce         bool           `yaml:"enforce"`
	FailThreshold   string         `yaml:"fail_threshold"`
	IgnorePaths     []string       `yaml:"ignore_paths"`
	SeverityWeights map[string]int `yaml:"severity_weights"`
}

// validFailThresholds is the set of accepted fail_threshold values
// (case-sensitive).
var validFailThresholds = map[string]struct{}{
	"CRITICAL": {},
	"HIGH":     {},
	"MEDIUM":   {},
}

// DefaultConfig returns the configuration used when .pipelineguard.yml is absent
// (and, field by field, whenever the file omits a value).
//
// SeverityWeights is a fresh copy of scoring.DefaultSeverityWeights so callers
// — and the YAML merge in Load — never mutate the shared package-level map.
func DefaultConfig() Config {
	weights := make(map[string]int, len(scoring.DefaultSeverityWeights))
	for sev, w := range scoring.DefaultSeverityWeights {
		weights[sev] = w
	}

	return Config{
		Scanners: ScannersConfig{
			Gitleaks: true,
			Trivy:    true,
			Semgrep:  false,
		},
		Enforce:         false,
		FailThreshold:   "CRITICAL",
		IgnorePaths:     nil,
		SeverityWeights: weights,
	}
}

// Load reads the YAML config file at path.
//
// The file is optional: if it does not exist, Load returns DefaultConfig() and
// no error. When it does exist, parsing starts from DefaultConfig() and
// yaml.Unmarshal merges the file on top, so any field the file omits keeps its
// default value — including individual keys of the scanners block and of
// severity_weights (yaml.v3 decodes into the existing struct/map without
// clearing untouched entries; this is covered by config_test.go).
//
// FailThreshold must be one of "CRITICAL", "HIGH", "MEDIUM" (case-sensitive);
// anything else is a descriptive error. Malformed YAML is returned as an error
// wrapped with %w, never a panic.
func Load(path string) (Config, error) {
	cfg := DefaultConfig()

	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return Config{}, fmt.Errorf("reading config file %q: %w", path, err)
	}

	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return Config{}, fmt.Errorf("parsing config file %q: %w", path, err)
	}

	if _, ok := validFailThresholds[cfg.FailThreshold]; !ok {
		return Config{}, fmt.Errorf(
			"invalid fail_threshold %q: must be one of CRITICAL, HIGH, MEDIUM", cfg.FailThreshold)
	}

	return cfg, nil
}
