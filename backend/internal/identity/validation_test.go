package identity

import (
	"strings"
	"testing"
)

func TestProjectSlugPatternMatchesDatabaseConstraint(t *testing.T) {
	valid := []string{
		"abc",
		"a-1",
		"production",
		"api--canary",
		strings.Repeat("a", 64),
	}
	for _, slug := range valid {
		if !slugPattern.MatchString(slug) {
			t.Errorf("slugPattern rejected valid database slug %q", slug)
		}
	}

	invalid := []string{
		"",
		"a",
		"ab",
		"-abc",
		"abc-",
		"Uppercase",
		"with_underscore",
		strings.Repeat("a", 65),
	}
	for _, slug := range invalid {
		if slugPattern.MatchString(slug) {
			t.Errorf("slugPattern accepted invalid database slug %q", slug)
		}
	}
}
