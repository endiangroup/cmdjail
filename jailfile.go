package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
)

var (
	rsnMissingPlusOrMinus = "missing white (+) or black (-) list prefix"
	rsnNoMatcher          = "missing a matcher e.g. 'ls -al'"
)

type JailFileParserErr struct {
	Location   string
	LineNumber int
	Line       string
	Reason     string
}

func NewJailFileParserErr(c Config, lineNum int, line, reason string) JailFileParserErr {
	return JailFileParserErr{
		Location:   c.JailFile,
		LineNumber: lineNum,
		Line:       line,
		Reason:     reason,
	}
}

func (j JailFileParserErr) Error() string {
	locAndLine := fmt.Sprintf("%s:%d", j.Location, j.LineNumber)
	return fmt.Sprintf("parsing jail file: %s\n\t%s: %s", j.Reason, locAndLine, j.Line)
}

func (j *JailFileParserErr) Is(other error) bool {
	return strings.Contains(other.Error(), "parsing jail file")
}

var ErrEmptyJailFile = errors.New("empty jail file")

type JailFile struct {
	Allow []Matcher
	Deny  []Matcher
}

func parseJailFile(conf Config, f io.Reader) (JailFile, error) {
	scanner := bufio.NewScanner(f)

	var jf JailFile
	for i := 1; scanner.Scan(); i++ {
		originalLine := scanner.Text()
		trimmedLine := strings.TrimSpace(originalLine)

		if trimmedLine == "" || strings.HasPrefix(trimmedLine, "#") {
			continue // Skip empty lines and comments
		}

		matcher, ruleType, err := parseRuleLine(trimmedLine, i, conf)
		if err != nil {
			// Pass originalLine for more accurate error reporting if needed,
			// though NewJailFileParserErr uses the (trimmed) line it receives.
			return JailFile{}, err
		}

		switch ruleType {
		case '+':
			jf.Allow = append(jf.Allow, matcher)
		case '-':
			jf.Deny = append(jf.Deny, matcher)
		}
	}

	if len(jf.Allow) == 0 && len(jf.Deny) == 0 {
		return jf, ErrEmptyJailFile
	}

	if err := scanner.Err(); err != nil {
		return JailFile{}, err
	}

	return jf, nil
}

// parseRuleLine processes a single non-empty, non-comment line from the jail file.
// It returns the parsed Matcher, the rule type ('+' or '-'), and any error.
func parseRuleLine(line string, lineNumber int, conf Config) (Matcher, byte, error) {
	if !strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "+") {
		return nil, 0, NewJailFileParserErr(conf, lineNumber, line, rsnMissingPlusOrMinus)
	}

	ruleType := line[0]
	ruleDefinition := strings.TrimSpace(line[1:])

	if ruleDefinition == "" {
		return nil, 0, NewJailFileParserErr(conf, lineNumber, line, rsnNoMatcher)
	}

	var m Matcher
	var err error
	baseMatcher := newMatcher(line, conf.JailFile, lineNumber) // Pass the full original line for Raw()

	if strings.HasPrefix(ruleDefinition, "'") {
		m = NewLiteralMatcher(baseMatcher, strings.TrimPrefix(ruleDefinition, "'"))
	} else if strings.HasPrefix(ruleDefinition, "r'") {
		m, err = NewRegexMatcher(baseMatcher, strings.TrimPrefix(ruleDefinition, "r'"))
		if err != nil {
			return nil, 0, NewJailFileParserErr(conf, lineNumber, line, err.Error())
		}
	} else {
		m = NewCmdMatcher(baseMatcher, ruleDefinition, conf.ShellCmd)
	}

	return m, ruleType, nil
}

func errIsAny(target error, errs ...error) bool {
	for _, err := range errs {
		if errors.Is(target, err) {
			return true
		}
	}

	return false
}
