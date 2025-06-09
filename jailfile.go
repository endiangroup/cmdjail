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
		line := scanner.Text()
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if !strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "+") {
			return jf, NewJailFileParserErr(conf, i, line, rsnMissingPlusOrMinus)
		}

		rule := strings.TrimSpace(line[1:])
		if rule == "" {
			return jf, NewJailFileParserErr(conf, i, line, rsnNoMatcher)
		}

		var err error
		var m Matcher
		if strings.HasPrefix(rule, "'") {
			m = NewLiteralMatcher(newMatcher(line, conf.JailFile, i), strings.TrimPrefix(rule, "'"))
		} else if strings.HasPrefix(rule, "r'") {
			m, err = NewRegexMatcher(newMatcher(line, conf.JailFile, i), strings.TrimPrefix(rule, "r'"))
			if err != nil {
				return jf, NewJailFileParserErr(conf, i, line, err.Error())
			}
		} else {
			m = NewCmdMatcher(newMatcher(line, conf.JailFile, i), rule, conf.ShellCmd)
		}

		switch line[0] {
		case '+':
			jf.Allow = append(jf.Allow, m)
		case '-':
			jf.Deny = append(jf.Deny, m)
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

func errIsAny(target error, errs ...error) bool {
	for _, err := range errs {
		if errors.Is(target, err) {
			return true
		}
	}

	return false
}
