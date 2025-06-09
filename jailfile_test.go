package main

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestItFailsToParseTheJailFile(t *testing.T) {
	t.Run("empty jail file", func(t *testing.T) {
		b := bytes.NewBuffer([]byte{})

		_, err := parseJailFile(NoConfig, b)

		assert.Equal(t, ErrEmptyJailFile, err)
	})
	t.Run("single newline character", func(t *testing.T) {
		b := bytes.NewBuffer([]byte{'\n'})

		_, err := parseJailFile(NoConfig, b)

		assert.Equal(t, ErrEmptyJailFile, err)
	})
	t.Run("multiple newline character", func(t *testing.T) {
		b := bytes.NewBuffer([]byte{'\n', '\n', '\n'})

		_, err := parseJailFile(NoConfig, b)

		assert.Equal(t, ErrEmptyJailFile, err)
	})
	t.Run("single line doesn't start with + or -", func(t *testing.T) {
		b := bytes.NewBuffer([]byte("ls -al"))
		expErr := JailFileParserErr{
			Location:   "/tmp/.cmd.jail",
			LineNumber: 1,
			Line:       "ls -al",
			Reason:     rsnMissingPlusOrMinus,
		}

		_, err := parseJailFile(Config{JailFile: "/tmp/.cmd.jail"}, b)

		assert.Equal(t, expErr, err)
	})
	t.Run("single line with plus or minus and no filter command", func(t *testing.T) {
		tests := []string{
			"+",
			"-",
			" +",
			" -",
			"+ ",
			"- ",
			" + ",
			" - ",
		}

		for _, test := range tests {
			b := bytes.NewBuffer([]byte(test))
			expErr := JailFileParserErr{
				Location:   "/tmp/.cmd.jail",
				LineNumber: 1,
				Line:       strings.TrimSpace(test),
				Reason:     rsnNoMatcher,
			}

			_, err := parseJailFile(Config{JailFile: "/tmp/.cmd.jail"}, b)

			assert.Equal(t, expErr, err)
		}
	})
	t.Run("invalid regex", func(t *testing.T) {
		b := bytes.NewBuffer([]byte("+ r'['"))
		_, err := parseJailFile(Config{JailFile: "/tmp/.cmd.jail"}, b)

		assert.Error(t, err)
		assert.IsType(t, JailFileParserErr{}, err)
		assert.Contains(t, err.Error(), "error parsing regexp")
	})
}

func TestItSuccesfullyParsesTheJailFile(t *testing.T) {
	t.Run("single cmd", func(t *testing.T) {
		tests := []string{
			"+ cmd",
			"+ cat /tmp/some.file",
			"- cmd",
			"- cat /tmp/some.file",
		}

		for _, test := range tests {
			t.Run(test, func(t *testing.T) {
				b := bytes.NewBuffer([]byte(test))

				jf, err := parseJailFile(NoConfig, b)
				assert.NoError(t, err)

				if test[0] == byte('+') {
					assert.Equal(t, test[2:], jf.Allow[0].(CmdMatcher).cmd)
				} else {
					assert.Equal(t, test[2:], jf.Deny[0].(CmdMatcher).cmd)
				}
			})
		}
	})
	t.Run("single literal match", func(t *testing.T) {
		tests := []string{
			"+ 'cmd",
			"+ 'cat /tmp/some.file",
			"- 'cmd",
			"- 'cat /tmp/some.file",
		}

		for _, test := range tests {
			t.Run(test, func(t *testing.T) {
				b := bytes.NewBuffer([]byte(test))

				jf, err := parseJailFile(NoConfig, b)
				assert.NoError(t, err)

				if test[0] == byte('+') {
					assert.Equal(t, test[3:], jf.Allow[0].(LiteralMatcher).str)
				} else {
					assert.Equal(t, test[3:], jf.Deny[0].(LiteralMatcher).str)
				}
			})
		}
	})
	t.Run("single regex match", func(t *testing.T) {
		tests := []string{
			"+ r'^cmd",
			"+ r'^cat /tmp/some.file",
			"- r'^cmd",
			"- r'^cat /tmp/some.file",
		}

		for _, test := range tests {
			t.Run(test, func(t *testing.T) {
				b := bytes.NewBuffer([]byte(test))

				jf, err := parseJailFile(NoConfig, b)
				assert.NoError(t, err)

				if test[0] == byte('+') {
					assert.Equal(t, test, jf.Allow[0].(RegexMatcher).raw)
				} else {
					assert.Equal(t, test, jf.Deny[0].(RegexMatcher).raw)
				}
			})
		}
	})
	t.Run("with comments and blank lines", func(t *testing.T) {
		content := `

	# This is a comment
	+ 'ls -l

	- r'^rm
	# Another comment
	`

		b := bytes.NewBufferString(content)
		jf, err := parseJailFile(NoConfig, b)

		assert.NoError(t, err)
		assert.Len(t, jf.Allow, 1)
		assert.Len(t, jf.Deny, 1)
		assert.Equal(t, "ls -l", jf.Allow[0].(LiteralMatcher).str)
		assert.Equal(t, "+ 'ls -l", jf.Allow[0].Raw())
		assert.Equal(t, "- r'^rm", jf.Deny[0].Raw())
	})
}

func TestCmdMatcher_Matches(t *testing.T) {
	m := newMatcher("", "/tmp/.cmd.jail", 1)
	shellCmd := []string{"/bin/sh", "-c"}

	t.Run("Success on exit 0", func(t *testing.T) {
		matcher := NewCmdMatcher(m, "true", shellCmd)
		matches, err := matcher.Matches("ls")
		assert.NoError(t, err)
		assert.True(t, matches)
	})
	t.Run("No match on non-zero exit", func(t *testing.T) {
		matcher := NewCmdMatcher(m, "false", shellCmd)
		matches, err := matcher.Matches("any command")
		assert.NoError(t, err)
		assert.False(t, matches)
	})

	t.Run("Error on command not found", func(t *testing.T) {
		matcher := NewCmdMatcher(m, "/path/to/nonexistent/script", shellCmd)
		matches, err := matcher.Matches("any command")

		assert.False(t, matches)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "exit status 127")
	})

	t.Run("Error on non-executable script", func(t *testing.T) {
		tmpfile, err := os.CreateTemp("", "test-script")
		assert.NoError(t, err)
		defer os.Remove(tmpfile.Name())

		tmpfile.WriteString("#!/bin/sh\nexit 0")
		tmpfile.Close()
		os.Chmod(tmpfile.Name(), 0o644)

		matcher := NewCmdMatcher(m, tmpfile.Name(), shellCmd)
		matches, err := matcher.Matches("any command")

		assert.Error(t, err)
		assert.False(t, matches)
		if err != nil {
			assert.Contains(t, err.Error(), "exit status 126")
		}
	})
}
