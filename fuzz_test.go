package main

import (
	"bytes"
	"testing"
)

func FuzzParseJailFile(f *testing.F) {
	f.Add([]byte("+ 'ls -l\n- r'^rm"))
	f.Add([]byte("# A comment\n\n+ 'whoami"))
	f.Add([]byte("malformed line"))
	f.Add([]byte("+"))
	f.Add([]byte("- r'['")) // Invalid regex

	f.Fuzz(func(t *testing.T, data []byte) {
		conf := Config{
			JailFile: "fuzz.jail",
			ShellCmd: []string{"bash", "-c"},
		}

		// The function should never panic. It should either return a valid
		// JailFile or a parsing error.
		_, _ = parseJailFile(conf, bytes.NewReader(data))
	})
}

func FuzzEvaluateCmd(f *testing.F) {
	jailFileContent := `
+ 'whoami
+ 'ls -l
- r'^rm
+ grep -qE '^(date|uptime)'
`
	conf := Config{
		JailFile: "fuzz.jail",
		ShellCmd: []string{"bash", "-c"},
	}
	jailFile, err := parseJailFile(conf, bytes.NewReader([]byte(jailFileContent)))
	if err != nil {
		f.Fatalf("Failed to parse seed jail file: %v", err)
	}

	f.Add("ls -l")
	f.Add("whoami")
	f.Add("rm -rf /")
	f.Add("date")
	f.Add("uptime")
	f.Add("echo; whoami")
	f.Add("'; whoami #")
	f.Add(string([]byte{0x00, 0x01, 0x02})) // Non-printable characters

	f.Fuzz(func(t *testing.T, intentCmd string) {
		// The evaluation function should never panic, regardless of the
		// intent command string. It should always return a valid CheckResult.
		_ = evaluateCmd(intentCmd, jailFile)
	})
}
