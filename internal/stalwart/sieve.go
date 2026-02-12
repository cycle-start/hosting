package stalwart

import "strings"

// GenerateForwardScript generates a Sieve script with redirect rules.
// Returns an empty string if there are no forwards.
// Uses :copy extension when keep_copy is true (delivers locally and forwards),
// and plain redirect when keep_copy is false (only forwards).
func GenerateForwardScript(forwards []ForwardRule) string {
	if len(forwards) == 0 {
		return ""
	}

	var b strings.Builder

	// Check if any forward needs :copy.
	needsCopy := false
	for _, f := range forwards {
		if f.KeepCopy {
			needsCopy = true
			break
		}
	}

	if needsCopy {
		b.WriteString("require [\"copy\"];\n")
	}

	for _, f := range forwards {
		if f.KeepCopy {
			b.WriteString("redirect :copy \"")
		} else {
			b.WriteString("redirect \"")
		}
		b.WriteString(f.Destination)
		b.WriteString("\";\n")
	}

	return b.String()
}
