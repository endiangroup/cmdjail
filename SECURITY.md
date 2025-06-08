# Security Vulnerability Analysis

This document outlines security vulnerabilities identified in the `cmdjail` codebase. This analysis ignores the inherent risks of the `CmdMatcher` and the execution of the `intentCmd` itself, and instead focuses on flaws that could allow an attacker to bypass the intended restrictions.

## 1. Shell Command Injection via Permissive Rules

This is the most critical vulnerability. The way `cmdjail` evaluates rules before execution creates a classic command injection scenario if the rules are not perfectly strict.

*   **The Flaw:** `cmdjail` validates the *entire command string* against a rule. If the rule matches, the *entire, un-sanitized command string* is passed to `bash -c`. A permissive rule (e.g., one that only checks for a command prefix) can allow an attacker to append a second, malicious command.

*   **Attack Scenario:**
    1.  An administrator creates a seemingly safe rule to allow users to check the status of a service:
        ```diff
        # .cmd.jail
        # Allow checking the status of 'myservice'
        + r'^systemctl status myservice'
        ```
    2.  The attacker provides the following intent command:
        ```sh
        cmdjail -- 'systemctl status myservice; /bin/bash'
        ```
    3.  **Bypass:**
        *   The `RegexMatcher` checks if the string `systemctl status myservice; /bin/bash` matches the pattern `^systemctl status myservice`. It does.
        *   The rule is considered a match, and `cmdjail` allows the command.
        *   `main.go` executes `bash -c "systemctl status myservice; /bin/bash"`.
        *   The shell first runs the status command and then, because of the semicolon (`;`), executes `/bin/bash`, giving the attacker a full, unrestricted shell.

*   **Mitigation:** The application should parse the command string into a command and its arguments *before* matching. The matching logic should then be performed on the parsed components, not the raw string. This prevents shell metacharacters from being interpreted.

## 2. Brittle and Bypassable Safety Checks

The `checkCmdSafety` function in `config.go` is intended to prevent the user from manipulating the jail file, the binary, or the log file. However, its implementation using `strings.Contains` is easy to bypass.

*   **The Flaw:** The check looks for exact substrings like `.cmd.jail` or `cmdjail`. An attacker can trivially obfuscate these strings in a way that the check misses but the shell correctly interprets.

*   **Attack Scenario:**
    1.  The jail file allows the `cat` command:
        ```diff
        # .cmd.jail
        + r'^cat '
        ```
    2.  The attacker wants to read the contents of the jail file to discover all the rules. A direct attempt is blocked:
        ```sh
        $ cmdjail -- 'cat .cmd.jail'
        [error] attempting to manipulate: .cmd.jail. Aborted
        ```
    3.  **Bypass:** The attacker uses simple shell quoting to break up the string:
        ```sh
        cmdjail -- 'cat ".cmd."jail'
        ```
    4.  The `checkCmdSafety` function checks the string `cat ".cmd."jail` for the substring `.cmd.jail`. It doesn't find it, so the check passes. The command is then allowed by the `cat` rule and executed by the shell, which concatenates the quoted parts and reads the file.

*   **Mitigation:** Safety checks should operate on canonical, absolute file paths. The application should resolve any file paths in the intent command to their absolute form and then perform checks. Simple substring matching is insufficient.

## 3. Interactive Shell Escape via Ctrl+D

In interactive shell mode, the application logic for handling `exit` or `quit` is sound—the command must be explicitly allowed by the jail file. However, there is a universal bypass.

*   **The Flaw:** The interactive shell in `runShell` reads from `os.Stdin` using a `bufio.Scanner`. When the user presses `Ctrl+D`, the input stream (stdin) is closed. This causes `scanner.Scan()` to return `false`, cleanly terminating the loop and exiting the shell with code 0, regardless of the jail file rules.

*   **Attack Scenario:**
    1.  An administrator has a strict jail file that does *not* include an allow rule for `exit` or `quit`, intending to trap the user in the `cmdjail` shell.
        ```diff
        # .cmd.jail
        + 'whoami'
        + 'date'
        ```
    2.  The user enters the shell. Typing `exit` is blocked as expected.
    3.  **Bypass:** The user simply presses `Ctrl+D`. The `cmdjail` process exits immediately.

*   **Impact:** While this doesn't lead to arbitrary code execution, it bypasses the intended control to keep the user within the restricted shell.

## 4. Time-of-Check to Time-of-Use (TOCTOU) Race Condition

This vulnerability applies if a `CmdMatcher` script validates a resource (like a file) that the `intentCmd` will later use.

*   **The Flaw:** There is a delay between when the `CmdMatcher` script checks a resource and when the `intentCmd` actually uses it. An attacker can alter the resource in that window.

*   **Attack Scenario:**
    1.  A jail file uses a script to ensure `cat` is only used on "safe" files in `/tmp`.
        ```diff
        # .cmd.jail
        # validator.sh checks if the file path given is in /tmp/safedir
        + /usr/local/bin/validator.sh
        ```
    2.  The attacker runs a command to read a safe file but prepares a background process to exploit the race condition.
        ```sh
        cmdjail -- 'cat /tmp/safedir/legit.txt'
        ```
    3.  **Bypass:**
        *   `CmdMatcher` runs `validator.sh`. The script inspects the command, sees `/tmp/safedir/legit.txt`, and exits 0 (success).
        *   In the nanoseconds after `validator.sh` exits but before `cmdjail` executes the `cat` command, the attacker's background process replaces `/tmp/safedir/legit.txt` with a symbolic link to `/etc/shadow`.
        *   `cmdjail` executes `cat /tmp/safedir/legit.txt`, which now reads the contents of `/etc/shadow`.

*   **Mitigation:** This is notoriously difficult to fix. The best approach is to avoid check-then-act patterns on shared resources. The matcher script should, if possible, perform the entire action itself rather than just validating parameters for another command.
