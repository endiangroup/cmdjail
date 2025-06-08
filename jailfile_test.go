package main

import (
	"bytes"
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
