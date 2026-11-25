package cli

import (
	"slices"
	"testing"
)

// each verb answers to its short names
func TestCommandAliases(t *testing.T) {
	want := map[string][]string{
		"add":      {"a", "ad", "in", "install"},
		"sync":     {"s", "sy", "up", "update"},
		"remove":   {"r", "rm", "rem", "un", "unin", "uninstall"},
		"list":     {"l", "li", "ls"},
		"outdated": {"o", "ou", "out", "stale"},
		"find":     {"f", "fi", "se", "search"},
		"tidy":     {"t", "ti", "cl", "clean"},
		"pull":     {"p", "refresh", "rf"},
		"doctor":   {"doc"},
		"upgrade":  {"ug", "upg"},
	}
	root := NewRootCmd()
	for name, aliases := range want {
		for _, a := range append([]string{name}, aliases...) {
			c, _, err := root.Find([]string{a})
			if err != nil || c.Name() != name {
				t.Errorf("%q should reach %q, got %v (%v)", a, name, c.Name(), err)
			}
		}
	}
	for _, c := range root.Commands() {
		if _, ok := want[c.Name()]; !ok && len(c.Aliases) > 0 && !slices.Contains([]string{"completion", "help"}, c.Name()) {
			t.Errorf("%q has aliases not in the table: %v", c.Name(), c.Aliases)
		}
	}
}
