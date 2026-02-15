package request

import (
	"fmt"
	"regexp"
)

var envVarNameRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,127}$`)

type SetWebrootEnvVars struct {
	Vars []EnvVarEntry `json:"vars" validate:"required,dive"`
}

type EnvVarEntry struct {
	Name   string `json:"name" validate:"required"`
	Value  string `json:"value"`
	Secret bool   `json:"secret"`
}

// Validate checks that all var names match the allowed pattern.
func (r *SetWebrootEnvVars) Validate() error {
	seen := make(map[string]bool, len(r.Vars))
	for _, v := range r.Vars {
		if !envVarNameRe.MatchString(v.Name) {
			return fmt.Errorf("invalid env var name %q: must match %s", v.Name, envVarNameRe.String())
		}
		if seen[v.Name] {
			return fmt.Errorf("duplicate env var name %q", v.Name)
		}
		seen[v.Name] = true
	}
	return nil
}
