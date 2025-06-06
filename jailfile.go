package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
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

type Matcher interface {
	Matches(string) (bool, error)
}

type matcher struct {
	raw        string
	lineNumber int
	jailFile   string
}

func newMatcher(raw, jailFile string, lineNumber int) matcher {
	return matcher{
		raw:        raw,
		lineNumber: lineNumber,
		jailFile:   jailFile,
	}
}

type LiteralMatcher struct {
	matcher
	str string
}

func NewLiteralMatcher(m matcher, s string) LiteralMatcher {
	s = strings.TrimPrefix(s, "'")

	return LiteralMatcher{
		matcher: m,
		str:     s,
	}
}

func (m LiteralMatcher) Matches(intentCmd string) (bool, error) {
	if m.str == intentCmd {
		return true, nil
	}

	return false, nil
}

type RegexMatcher struct {
	matcher
	re *regexp.Regexp
}

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

func (r RegexMatcher) Matches(intentCmd string) (bool, error) {
	return r.re.MatchString(intentCmd), nil
}

type CmdMatcher struct {
	matcher
	cmd string
}

func NewCmdMatcher(m matcher, c string) CmdMatcher {
	return CmdMatcher{
		matcher: m,
		cmd:     c,
	}
}

func (c CmdMatcher) Matches(intentCmd string) (bool, error) {
	cmd := exec.Command("bash", "-c", c.cmd)
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
		printLogDebug(os.Stdout, "%s:%d: matcher '%s:\n%s", c.jailFile, c.lineNumber, c.raw, exerr.Stderr)
		return false, nil
	}
	printLogErr(os.Stdout, "%s:%d: matcher '%s:%s", c.jailFile, c.lineNumber, c.raw, err.Error())
	return false, err
}

type JailFile struct {
	Allow []Matcher
	Deny  []Matcher
}

func parseJailFile(conf Config, f io.Reader) (JailFile, error) {
	scanner := bufio.NewScanner(f)

	var jf JailFile
	for i := 1; scanner.Scan(); i++ {
		line := scanner.Text()
		if line != "" {
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "+") {
				return jf, NewJailFileParserErr(conf, i, line, rsnMissingPlusOrMinus)
			}

			if len(line) < 2 {
				return jf, NewJailFileParserErr(conf, i, line, rsnNoMatcher)
			}

			var err error
			var m Matcher
			if line[2] == '\'' {
				m = NewLiteralMatcher(newMatcher(line, conf.JailFile, i), line[3:])
			} else if line[2] == 'r' && line[3] == '\'' {
				m, err = NewRegexMatcher(newMatcher(line, conf.JailFile, i), line[4:])
				if err != nil {
					return jf, NewJailFileParserErr(conf, i, line, err.Error())
				}
			} else {
				m = NewCmdMatcher(newMatcher(line, conf.JailFile, i), line[2:])
			}

			switch line[0] {
			case '+':
				jf.Allow = append(jf.Allow, m)
			case '-':
				jf.Deny = append(jf.Deny, m)
			}
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
