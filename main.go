package main

import (
	"errors"
	"io/fs"
	"os"
	"os/exec"
)

func main() {
	setLoggerToSyslog()

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

	jailFileFd, err := os.Open(conf.JailFile)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			printLogErr(os.Stderr, "finding jail file: %s", conf.JailFile)
		} else {
			printLogErr(os.Stderr, "opening jail file: %s: %s", conf.JailFile, err.Error())
		}
		os.Exit(1)
	}

	jailFile, err := parseJailFile(conf, jailFileFd)
	if err != nil {
		if errors.Is(err, ErrEmptyJailFile) {
			printLogErr(os.Stderr, "empty jail file: %s", conf.JailFile)
		} else {
			printLogErr(os.Stderr, "%s", err.Error())
		}
		os.Exit(1)
	}

	// TODO: If only blacklist, implicity run when no match
	for _, deny := range jailFile.Deny {
		match, err := deny.Matches(conf.Cmd)
		if err != nil {
			logErr("running matcher: %s", err.Error())
			os.Exit(1)
		}
		if match {
			logWarn("blocked blacklisted intent cmd: %s", conf.Cmd)
			os.Exit(77)
		}
	}
	for _, allow := range jailFile.Allow {
		match, err := allow.Matches(conf.Cmd)
		if err != nil {
			logErr("running matcher: %s", err.Error())
			os.Exit(1)
		}
		if match {
			os.Exit(runCmd(conf.Cmd))
		}
	}

	logWarn("implicitly blocked intent cmd: %s", conf.Cmd)
	os.Exit(77)
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
