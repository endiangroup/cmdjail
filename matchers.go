package main

import (
	"os"
	"os/exec"
	"regexp"
	"strings"
)

// Matcher defines the interface for different rule matching strategies.
type Matcher interface {
	Matches(string) (bool, error)
	Raw() string
}

// matcher is a base struct embedded by concrete matcher types.
// It holds common information about the rule's origin.
type matcher struct {
	raw        string
	lineNumber int
	jailFile   string
}

// Raw returns the original raw string of the rule.
func (m matcher) Raw() string {
	return m.raw
}

// newMatcher creates a new matcher base.
func newMatcher(raw, jailFile string, lineNumber int) matcher {
	return matcher{
		raw:        raw,
		lineNumber: lineNumber,
		jailFile:   jailFile,
	}
}

// LiteralMatcher performs an exact string comparison.
type LiteralMatcher struct {
	matcher
	str string
}

// NewLiteralMatcher creates a new LiteralMatcher.
// It trims the leading single quote from the rule string.
func NewLiteralMatcher(m matcher, s string) LiteralMatcher {
	s = strings.TrimPrefix(s, "'")

	return LiteralMatcher{
		matcher: m,
		str:     s,
	}
}

// Matches checks if the intentCmd exactly matches the rule string.
func (m LiteralMatcher) Matches(intentCmd string) (bool, error) {
	printLogDebug(os.Stdout, "LiteralMatcher: comparing intent '%s' with rule string '%s'", intentCmd, m.str)
	if m.str == intentCmd {
		return true, nil
	}

	return false, nil
}

// RegexMatcher matches the intent command against a regular expression.
type RegexMatcher struct {
	matcher
	re *regexp.Regexp
}

// NewRegexMatcher creates a new RegexMatcher.
// It trims the leading "r'" from the rule string and compiles the regex.
func NewRegexMatcher(m matcher, re string) (RegexMatcher, error) {
	re = strings.TrimPrefix(re, "r'")

	r, err := regexp.Compile(re)
	if err != nil {
		return RegexMatcher{}, err
	}

	return RegexMatcher{
		matcher: m,
		re:      r,
	}, nil
}

// Matches checks if the intentCmd matches the compiled regular expression.
func (r RegexMatcher) Matches(intentCmd string) (bool, error) {
	printLogDebug(os.Stdout, "RegexMatcher: matching intent '%s' against pattern '%s'", intentCmd, r.re.String())
	return r.re.MatchString(intentCmd), nil
}

// CmdMatcher executes an external command/script to determine a match.
// The intent command is passed to the script's stdin.
// A match occurs if the script exits with code 0.
type CmdMatcher struct {
	matcher
	cmd      string
	shellCmd []string
}

// NewCmdMatcher creates a new CmdMatcher.
func NewCmdMatcher(m matcher, c string, shellCmd []string) CmdMatcher {
	return CmdMatcher{
		matcher:  m,
		cmd:      c,
		shellCmd: shellCmd,
	}
}

// Matches executes the matcher command and checks its exit status.
func (c CmdMatcher) Matches(intentCmd string) (bool, error) {
	printLogDebug(os.Stdout, "CmdMatcher: executing '%s' with stdin: %s", c.cmd, intentCmd)

	cmd := exec.Command(c.shellCmd[0], append(c.shellCmd[1:], c.cmd)...)
	w, err := cmd.StdinPipe()
	if err != nil {
		return false, err
	}
	if err = cmd.Start(); err != nil {
		return false, err
	}
	w.Write([]byte(intentCmd))
	if err = w.Close(); err != nil {
		return false, err
	}
	err = cmd.Wait()
	if err == nil {
		return true, nil
	}

	if exerr, ok := err.(*exec.ExitError); ok {
		if exerr.ExitCode() == 1 { // Specific exit code 1 means "no match" but not an operational error.
			return false, nil
		}
		// For other non-zero exit codes, log the error details.
		printLogErr(os.Stderr, "%s:%d: matcher '%s': %s\n%s", c.jailFile, c.lineNumber, c.raw, exerr.Error(), exerr.Stderr)
	}
	// Return the error for further processing or logging by the caller.
	return false, err
}
