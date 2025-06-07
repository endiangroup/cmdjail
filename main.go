package main

import (
	"errors"
	"io"
	"io/fs"
	"log"
	"os"
	"os/exec"
)

func main() {
	conf := getConfig()
	printLogDebug(os.Stdout, "config loaded: %+v\n", conf)
	jailFile := getJailFile(conf)
	printLogDebug(os.Stdout, "jail file loaded: %d allow rules, %d deny rules\n", len(jailFile.Allow), len(jailFile.Deny))
	printLogDebug(os.Stdout, "evaluating intent command: %s\n", conf.IntentCmd)

	// Check blacklisted commands first
	for i, deny := range jailFile.Deny {
		printLogDebug(os.Stdout, "checking deny rule #%d: %s\n", i+1, deny.Raw())
		match, err := deny.Matches(conf.IntentCmd)
		if err != nil {
			logErr("running matcher: %s", err.Error())
			os.Exit(1)
		}
		if match {
			logWarn("blocked blacklisted intent cmd: %s", conf.IntentCmd)
			os.Exit(77)
		}
	}

	// If there are no whitelist entries assume blacklist behaviour,
	// any intent cmd that doesn't match an explicit deny matcher is
	// permitted.
	if len(jailFile.Allow) == 0 {
		os.Exit(runCmd(conf.IntentCmd))
	}

	// Check whitelisted commands
	for i, allow := range jailFile.Allow {
		printLogDebug(os.Stdout, "checking allow rule #%d: %s\n", i+1, allow.Raw())
		match, err := allow.Matches(conf.IntentCmd)
		if err != nil {
			logErr("running matcher: %s", err.Error())
			os.Exit(1)
		}
		if match {
			printLogDebug(os.Stdout, "command explicitly allowed, executing\n")
			os.Exit(runCmd(conf.IntentCmd))
		}
	}

	logWarn("implicitly blocked intent cmd: %s", conf.IntentCmd)
	os.Exit(77)
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

	if isStdinSet() {
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
