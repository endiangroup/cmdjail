package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"strings"
)

func main() {
	conf := getConfig()
	printLogDebug(os.Stdout, "config loaded: %+v\n", conf)

	if conf.RecordFile != "" {
		printLogWarn(os.Stderr, "cmdjail running in record mode")
		os.Exit(recordIntentCmd(conf))
	}

	jailFile := getJailFile(conf)
	if conf.Shell {
		os.Exit(runShell(conf, jailFile))
	}

	os.Exit(evaluateAndRun(conf.IntentCmd, jailFile))
}

func recordIntentCmd(conf Config) int {
	printLogDebug(os.Stdout, "record mode enabled, recording to: %s\n", conf.RecordFile)
	if err := appendRuleToFile(conf.RecordFile, conf.IntentCmd); err != nil {
		printLogErr(os.Stderr, "appending to record file %s: %s", conf.RecordFile, err.Error())
		return 1
	}
	printLogDebug(os.Stdout, "appended rule to %s: + '%s'\n", conf.RecordFile, conf.IntentCmd)

	return runCmd(conf.IntentCmd)
}

func runShell(conf Config, jailFile JailFile) int {
	scanner := bufio.NewScanner(os.Stdin)

	fmt.Print("cmdjail> ")
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "exit" || line == "quit" {
			return 0
		}
		if line == "" {
			fmt.Print("cmdjail> ")
			continue
		}

		if err := checkCmdSafety(line, conf.Log); err != nil {
			printLogErr(os.Stderr, "%s\n", err.Error())
			fmt.Print("cmdjail> ")
			continue
		}

		evaluateAndRun(line, jailFile)
		fmt.Print("cmdjail> ")
	}

	if err := scanner.Err(); err != nil {
		printLogErr(os.Stderr, "reading from stdin: %v", err)
		return 1
	}
	fmt.Println() // Print a newline on exit (e.g., Ctrl+D)
	return 0
}

func evaluateAndRun(intentCmd string, jailFile JailFile) int {
	printLogDebug(os.Stdout, "evaluating intent command: %s\n", intentCmd)

	// Check blacklisted commands first
	for i, deny := range jailFile.Deny {
		printLogDebug(os.Stdout, "checking deny rule #%d: %s\n", i+1, deny.Raw())
		match, err := deny.Matches(intentCmd)
		if err != nil {
			logErr("running matcher: %s", err.Error())
			return 1
		}
		if match {
			logWarn("blocked blacklisted intent cmd: %s", intentCmd)
			return 77
		}
	}

	if len(jailFile.Allow) == 0 {
		return runCmd(intentCmd)
	}

	// Check whitelisted commands
	for i, allow := range jailFile.Allow {
		printLogDebug(os.Stdout, "checking allow rule #%d: %s\n", i+1, allow.Raw())
		match, err := allow.Matches(intentCmd)
		if err != nil {
			logErr("running matcher: %s", err.Error())
			return 1
		}
		if match {
			printLogDebug(os.Stdout, "command explicitly allowed, executing\n")
			return runCmd(intentCmd)
		}
	}

	logWarn("implicitly blocked intent cmd: %s", intentCmd)
	return 77
}

func getConfig() Config {
	conf, err := parseEnvAndFlags()
	if err != nil {
		printLogErr(os.Stderr, "%s", err.Error())
		if errIsAny(err,
			ErrCmdNotWrappedInQuotes,
			ErrJailFileManipulationAttempt,
			ErrJailBinaryManipulationAttempt,
			ErrJailLogManipulationAttempt,
			ErrShellModeWithRecord) {
			os.Exit(77)
		}
		os.Exit(1)
	}
	return conf
}

func getJailFile(conf Config) JailFile {
	var jailFileReader io.Reader
	var err error

	if isStdinSet() && !conf.Shell {
		jailFileReader = os.Stdin
	} else {
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

func isStdinSet() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		log.Fatal(err)
	}

	var hasStdin bool
	if fi.Mode()&os.ModeNamedPipe != 0 {
		hasStdin = true
	}
	return hasStdin
}

func runCmd(c string) int {
	cmd := exec.Command("bash", "-c", c)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		logErr("running intent cmd: %s: %s", c, err.Error())
		return 1
	}
	if err := cmd.Wait(); err != nil {
		logErr("running intent cmd: %s: %s", c, err.Error())
		if exerr, ok := err.(*exec.ExitError); ok {
			return exerr.ExitCode()
		}
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
