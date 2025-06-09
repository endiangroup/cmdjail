package main

import (
	"fmt"
	"os"
)

type CheckResult struct {
	Cmd     string
	Allowed bool
	Reason  string
	Matcher Matcher
}

// evaluateCmd checks an intent command against a jail file without executing it.
func evaluateCmd(intentCmd string, jailFile JailFile) CheckResult {
	printLogDebug(os.Stdout, "evaluating intent command: %s", intentCmd)

	// Check blacklisted commands first
	for i, deny := range jailFile.Deny {
		printLogDebug(os.Stdout, "checking deny rule #%d: %s", i+1, deny.Raw())
		match, err := deny.Matches(intentCmd)
		if err != nil {
			return CheckResult{Cmd: intentCmd, Allowed: false, Reason: fmt.Sprintf("error running matcher: %s", err.Error())}
		}
		if match {
			return CheckResult{Cmd: intentCmd, Allowed: false, Reason: "Matched deny rule", Matcher: deny}
		}
	}

	if len(jailFile.Allow) == 0 {
		// Blacklist only mode, command is allowed if not denied.
		return CheckResult{Cmd: intentCmd, Allowed: true, Reason: "No allow rules defined, command allowed by default"}
	}

	// Check whitelisted commands
	for i, allow := range jailFile.Allow {
		printLogDebug(os.Stdout, "checking allow rule #%d: %s", i+1, allow.Raw())
		match, err := allow.Matches(intentCmd)
		if err != nil {
			return CheckResult{Cmd: intentCmd, Allowed: false, Reason: fmt.Sprintf("error running matcher: %s", err.Error())}
		}
		if match {
			return CheckResult{Cmd: intentCmd, Allowed: true, Reason: "Matched allow rule", Matcher: allow}
		}
	}

	return CheckResult{Cmd: intentCmd, Allowed: false, Reason: "Implicitly blocked"}
}
