package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"strings"
)

var (
	version string
	commit  string
	date    string
)

func main() {
	conf := getConfig()
	printLogDebug(os.Stdout, "config loaded: %+v", conf)

	if conf.Version {
		printVersion()
		os.Exit(0)
	}

	// Check mode is a distinct mode of operation that exits early.
	if conf.CheckMode {
		printLogDebug(os.Stderr, "running in check mode")
		os.Exit(runCheckMode(conf, getJailFile(conf)))
	}

	if conf.Shell {
		var jailFile JailFile
		if conf.RecordFile != "" {
			printLogWarn(os.Stderr, "cmdjail shell mode recording to: %s", conf.RecordFile)
			// No jail file needed for record mode
		} else {
			jailFile = getJailFile(conf)
		}
		os.Exit(runShell(conf, jailFile))
	}

	// Single command mode
	if conf.RecordFile != "" {
		printLogWarn(os.Stderr, "cmdjail single-command record mode, recording to: %s", conf.RecordFile)
		os.Exit(recordIntentCmd(conf))
	}

	_, exitCode := evaluateAndRun(conf.IntentCmd, getJailFile(conf), conf.ShellCmd)
	os.Exit(exitCode)
}

func printVersion() {
	printMsg(os.Stderr, "version:%s", version)
	printMsg(os.Stderr, "commit:\t%s", commit)
	printMsg(os.Stderr, "built:\t%s", date)
}

func recordIntentCmd(conf Config) int {
	printLogDebug(os.Stdout, "record mode enabled, recording to: %s", conf.RecordFile)
	if err := appendRuleToFile(conf.RecordFile, conf.IntentCmd); err != nil {
		printLogErr(os.Stderr, "appending to record file %s: %s", conf.RecordFile, err.Error())
		return 1
	}
	printLogDebug(os.Stdout, "appended rule to %s: + '%s'", conf.RecordFile, conf.IntentCmd)

	return runCmd(conf.ShellCmd, conf.IntentCmd)
}

func runShell(conf Config, jailFile JailFile) int {
	scanner := bufio.NewScanner(os.Stdin)
	isRecordMode := conf.RecordFile != ""

	fmt.Print("cmdjail> ")
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			fmt.Print("cmdjail> ")
			continue
		}

		if err := checkCmdSafety(line, conf.Log); err != nil {
			printLogErr(os.Stderr, "%s", err.Error())
			fmt.Print("cmdjail> ")
			continue
		}

		if isRecordMode {
			if err := appendRuleToFile(conf.RecordFile, line); err != nil {
				printLogErr(os.Stderr, "appending to record file %s: %s", conf.RecordFile, err.Error())
			} else {
				printLogDebug(os.Stdout, "appended rule to %s: + '%s'", conf.RecordFile, line)
			}

			if line == "exit" || line == "quit" {
				return 0
			} else {
				runCmd(conf.ShellCmd, line)
			}

		} else {
			cmdWasAllowed, _ := evaluateAndRun(line, jailFile, conf.ShellCmd)

			if cmdWasAllowed && (line == "exit" || line == "quit") {
				return 0
			}
		}

		fmt.Print("cmdjail> ")
	}

	if err := scanner.Err(); err != nil {
		printLogErr(os.Stderr, "reading from stdin: %v", err)
		return 1
	}

	fmt.Println() // Print a newline on exit (e.g., Ctrl+D)
	return 0
}

func evaluateAndRun(intentCmd string, jailFile JailFile, shellCmd []string) (bool, int) {
	result := evaluateCmd(intentCmd, jailFile)

	if !result.Allowed {
		if result.Matcher != nil { // Matched a deny rule
			logWarn("blocked blacklisted intent cmd: %s", intentCmd)
		} else { // Implicitly blocked
			logWarn("implicitly blocked intent cmd: %s", intentCmd)
		}
		return false, 77
	}

	if result.Matcher != nil {
		printLogDebug(os.Stdout, "command explicitly allowed, executing")
	}
	return true, runCmd(shellCmd, intentCmd)
}

func runCheckMode(conf Config, jailFile JailFile) int {
	printMsg(os.Stdout, "Jail file '%s' syntax is valid.", conf.JailFile)

	var commands []string
	var err error
	source := "command line"

	if conf.CheckIntentCmdsFile != "" {
		var r io.Reader
		if conf.CheckIntentCmdsFile == "-" {
			source = "stdin"
			printMsg(os.Stdout, "\nReading commands from stdin to check...")
			r = os.Stdin
		} else {
			source = conf.CheckIntentCmdsFile
			file, fileErr := os.Open(conf.CheckIntentCmdsFile)
			if fileErr != nil {
				printLogErr(os.Stderr, "reading test file %s: %v", conf.CheckIntentCmdsFile, fileErr)
				return 1
			}
			defer file.Close()
			r = file
		}
		commands, err = readLines(r)
		if err != nil {
			printLogErr(os.Stderr, "reading test commands from %s: %v", source, err)
			return 1
		}
	} else if conf.IntentCmd != "" {
		commands = []string{conf.IntentCmd}
	}

	if len(commands) == 0 {
		printMsg(os.Stdout, "No commands provided to check. Exiting.")
		return 0
	}

	printMsg(os.Stdout, "\nTesting commands from %s...", source)
	blockedCount := 0
	for _, cmd := range commands {
		result := evaluateCmd(cmd, jailFile)
		if result.Allowed {
			printMsg(os.Stdout, "\n[ALLOWED] '%s'", cmd)
			printMsg(os.Stdout, "  Reason: %s", result.Reason)
			if result.Matcher != nil {
				printMsg(os.Stdout, "  Matcher: %s", result.Matcher.Raw())
			}
		} else {
			blockedCount++
			printMsg(os.Stdout, "\n[BLOCKED] '%s'", cmd)
			printMsg(os.Stdout, "  Reason: %s", result.Reason)
			if result.Matcher != nil {
				printMsg(os.Stdout, "  Matcher: %s", result.Matcher.Raw())
			}
		}
	}

	printMsg(os.Stdout, "\nCheck complete. %d/%d commands would be blocked.", blockedCount, len(commands))
	if blockedCount > 0 {
		return 1
	}
	return 0
}

// readLines reads from a reader and returns its lines as a slice of strings.
func readLines(r io.Reader) ([]string, error) {
	var lines []string
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

func getConfig() Config {
	conf, err := parseEnvAndFlags()
	if err != nil {
		printLogErr(os.Stderr, "%s", err.Error())
		if errIsAny(err,
			ErrCmdNotWrappedInQuotes,
			ErrJailFileManipulationAttempt,
			ErrJailBinaryManipulationAttempt,
			ErrJailLogManipulationAttempt) {
			os.Exit(77)
		}
		os.Exit(1)
	}
	return conf
}

func getJailFile(conf Config) JailFile {
	var jailFileReader io.Reader
	var err error

	if conf.JailFile == "-" && !conf.Shell {
		printLogDebug(os.Stdout, "reading jail file from: <stdin>")
		jailFileReader = os.Stdin
	} else {
		printLogDebug(os.Stdout, "reading jail file from: %s", conf.JailFile)
		jailFileReader, err = os.Open(conf.JailFile)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				printLogErr(os.Stderr, "finding jail file: %s", conf.JailFile)
			} else {
				printLogErr(os.Stderr, "opening jail file: %s: %s", conf.JailFile, err.Error())
			}
			os.Exit(1)
		}
	}

	jailFile, err := parseJailFile(conf, jailFileReader)
	if err != nil {
		if errors.Is(err, ErrEmptyJailFile) {
			printLogErr(os.Stderr, "empty jail file: %s", conf.JailFile)
		} else {
			printLogErr(os.Stderr, "%s", err.Error())
		}
		os.Exit(1)
	}

	return jailFile
}

func runCmd(shellCmd []string, c string) int {
	cmd := exec.Command(shellCmd[0], append(shellCmd[1:], c)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		printLogErr(os.Stderr, "running intent cmd: %s: %s", c, err.Error())
		return 1
	}
	if err := cmd.Wait(); err != nil {
		// Don't log error here as it's the command's exit status, not an application error.
		// The command's stderr is already piped.
		if exerr, ok := err.(*exec.ExitError); ok {
			return exerr.ExitCode()
		}
		// For other errors (not ExitError), it's an application-level issue.
		printLogErr(os.Stderr, "waiting for intent cmd: %s: %s", c, err.Error())
		return 1
	}

	return 0
}

func appendRuleToFile(filepath, intentCmd string) error {
	f, err := os.OpenFile(filepath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	rule := fmt.Sprintf("+ '%s\n", intentCmd)
	if _, err := f.WriteString(rule); err != nil {
		return err
	}
	return nil
}
