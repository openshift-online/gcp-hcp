package storage

import (
	"strings"
	"testing"
)

func TestGenerateName(t *testing.T) {
	t.Run("uses prefix", func(t *testing.T) {
		name := GenerateName("my-resource-")
		if !strings.HasPrefix(name, "my-resource-") {
			t.Errorf("expected prefix 'my-resource-', got %q", name)
		}
	})

	t.Run("appends 5-char suffix", func(t *testing.T) {
		prefix := "test-"
		name := GenerateName(prefix)
		suffix := strings.TrimPrefix(name, prefix)
		if len(suffix) != 5 {
			t.Errorf("expected 5-char suffix, got %d chars: %q", len(suffix), suffix)
		}
	})

	t.Run("generates distinct names", func(t *testing.T) {
		seen := make(map[string]bool)
		for range 100 {
			name := GenerateName("prefix-")
			if seen[name] {
				t.Fatalf("duplicate name generated: %q", name)
			}
			seen[name] = true
		}
	})
}
