package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEvaluateCmd(t *testing.T) {
	allowLs := NewLiteralMatcher(newMatcher("+ 'ls -l'", "", 1), "ls -l")
	denyRm, err := NewRegexMatcher(newMatcher("- r'^rm'", "", 2), "^rm")
	assert.NoError(t, err, "failed to compile regex for test")

	t.Run("Deny rule takes precedence over allow rule", func(t *testing.T) {
		allowRm := NewLiteralMatcher(newMatcher("+ 'rm -rf /'", "", 1), "rm -rf /")
		jailFile := JailFile{
			Allow: []Matcher{allowRm},
			Deny:  []Matcher{denyRm},
		}
		result := evaluateCmd("rm -rf /", jailFile)
		assert.False(t, result.Allowed)
		assert.Equal(t, "Matched deny rule", result.Reason)
		assert.Equal(t, denyRm, result.Matcher)
	})

	t.Run("Deny-only mode allows non-matching commands", func(t *testing.T) {
		jailFile := JailFile{
			Deny: []Matcher{denyRm},
		}
		result := evaluateCmd("ls -l", jailFile)
		assert.True(t, result.Allowed)
		assert.Equal(t, "No allow rules defined, command allowed by default", result.Reason)
	})

	t.Run("Allow-only mode blocks non-matching commands", func(t *testing.T) {
		jailFile := JailFile{
			Allow: []Matcher{allowLs},
		}
		result := evaluateCmd("whoami", jailFile)
		assert.False(t, result.Allowed)
		assert.Equal(t, "Implicitly blocked", result.Reason)
	})

	t.Run("Allow-only mode allows matching commands", func(t *testing.T) {
		jailFile := JailFile{
			Allow: []Matcher{allowLs},
		}
		result := evaluateCmd("ls -l", jailFile)
		assert.True(t, result.Allowed)
		assert.Equal(t, "Matched allow rule", result.Reason)
		assert.Equal(t, allowLs, result.Matcher)
	})

	t.Run("Mixed mode implicitly blocks non-matching commands", func(t *testing.T) {
		jailFile := JailFile{
			Allow: []Matcher{allowLs},
			Deny:  []Matcher{denyRm},
		}
		result := evaluateCmd("whoami", jailFile)
		assert.False(t, result.Allowed)
		assert.Equal(t, "Implicitly blocked", result.Reason)
	})

	t.Run("Matcher error results in denial", func(t *testing.T) {
		// This matcher will fail because the command doesn't exist
		errorMatcher := NewCmdMatcher(newMatcher("+ /nonexistent/command", "", 1), "/nonexistent/command", []string{"bash", "-c"})
		jailFile := JailFile{
			Allow: []Matcher{errorMatcher},
		}
		result := evaluateCmd("any", jailFile)
		assert.False(t, result.Allowed)
		assert.Contains(t, result.Reason, "error running matcher")
	})
}
