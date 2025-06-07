# cmdjail

A flexible, rule-based cli command filtering proxy.

`cmdjail` acts as an intermediary for executing shell commands. It evaluates a command string against a set of rules defined in a "jail file" and decides whether to execute or block it. This is particularly useful for restricting user actions in controlled environments, such as an `sshd` `ForceCommand` or a limited shell.

## Core Concepts

`cmdjail`'s behavior is governed by a few key concepts:

### 1. The Intent Command

This is the command string that a user _intends_ to run. `cmdjail` intercepts this command and evaluates it before execution.

### 2. The Jail File

This is a plain text file (default: `.cmd.jail`) that defines the filtering rules. Each line is a rule prefixed with either `+` (allow) or `-` (deny).

### 3. Matchers

Matchers are how `cmdjail` determines if an intent command corresponds to a rule. There are three types of matchers:

| Type        | Prefix | Syntax              | Description                                                                                                                                                   |
| :---------- | :----- | :------------------ | :------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| **Literal** | `'`    | `+ 'command string` | Performs an exact, case-sensitive string comparison. This is the simplest and safest matcher.                                                                 |
| **Regex**   | `r'`   | `+ r'^pattern$`     | Matches the intent command against a [Go-compatible regular expression](https://pkg.go.dev/regexp/syntax).                                                    |
| **Command** | ``     | `- /path/to/script` | Executes an external script or binary. The intent command is piped to the script's standard input. The match is successful if the script exits with code `0`. |

> **Security Warning:** The **Command Matcher** is powerful but can introduce vulnerabilities if the script it calls is insecure. Ensure that any script used as a matcher is well-vetted and not writable by the user being jailed.

### 4. Execution modes

`cmdjail` has 3 modes of operation

- **Deny Only**: Given only `-` (deny) rules in the jail file.
  1. It checks the intent command against all `-` (deny) rules sequentially.
  2. If any rule matches, the command is **blocked** and logged.
  3. If the intent command doesn't match any deny rule it is **executed!**.
- **Allow Only**: Given only `+` (allow) rules in the jail file.
  1. It checks the intent command against all `+` (allow) rules sequentially.
  2. If any rule matches, the command is **executed!**.
  3. If the intent command doesn't match any allow rule it is **blocked** and logged.
- **Mixed**: Given a mix of `-` (deny) and `+` (allow) rules in the jail file.
  1. It checks the intent command against all `-` (deny) rules sequentially first.
  2. If any `-` (deny) rule matches, the command is **blocked** and logged.
  3. It then checks the intent command against all `+` (allow) rules sequentially.
  4. If any `+` (allow) rule matches, the command is **executed!**.
  5. If the intent command doesn't match any `+` (allow) or `-` (deny) rule it is **blocked** and logged.

## Installing

You can build `cmdjail` from the source using the provided `Makefile`.

1.  **Clone the repository:**

    ```sh
    git clone https://github.com/endiangroup/cmdjail.git
    cd cmdjail
    ```

2.  **Build the binary:**
    You have two options for building:

- **For local development and testing:**
  This creates a binary in the `bin/` directory.

  ```sh
  make bin/cmdjail
  ```

- **For a specific platform (cross-compilation):**

  This creates a release-ready binary in the `build/` directory. You can specify the target operating system and architecture using the `GOOS` and `GOARCH` environment variables.

  ```sh
  # Build for Linux ARM64
  GOOS=linux GOARCH=arm64 make build

  # Build for Mac AMD64
  GOOS=darwin GOARCH=amd64 make build
  ```

  If `GOOS` and `GOARCH` are not set, it defaults to your current system's architecture.

3. **Install the binary:**
   Place the compiled `cmdjail` binary in a directory within your system's `PATH`.
   ```sh
   sudo mv bin/cmdjail /usr/local/bin/
   ```

## Usage

`cmdjail` can receive the intent command either from a command-line argument or an environment variable.

### Syntax

```
cmdjail [flags] -- 'command to run with arguments'
```

Flags

| Flag            | Shorthand | Environment Variable  | Description                                                                                                   |
| --------------- | --------- | --------------------- | ------------------------------------------------------------------------------------------------------------- |
| --jail-file     | -j        | CMDJAIL_JAILFILE      | Path to the jail file. Defaults to .cmd.jail in the same directory as the binary. Can also be piped to stdin. |
| --log-file      | -l        | CMDJAIL_LOG           | Path to a log file. Setting flag to empty string `""` sets to syslog. Default is no logging.                  |
| --env-reference | -e        | CMDJAIL_ENV_REFERENCE | Name of an environment variable containing the intent command (e.g., SSH_ORIGINAL_COMMAND).                   |
| --record        | -r        | CMDJAIL_RECORDFILE    | Transparently run the intent cmd and append it to the specified file as a literal allow rule.                 |
| --verbose       | -v        | CMDJAIL_VERBOSE       | Enable verbose logging for debugging.                                                                         |

### Jail File Examples (.cmd.jail)

#### Example 1: Simple Whitelist

**Goal:** Only allow the user to run `ls -l` and `whoami`.

This configuration uses whitelist mode. Only commands that are explicitly allowed will run.

`.cmd.jail`:

```diff
# Allow ls -l and whoami exactly.
+ 'ls -l
+ 'whoami
```

**Usage:**

```bash
# This command will be executed
$ cmdjail -- 'ls -l'
total 8
-rwxr-xr-x 1 user group 4096 Jun 1 12:00 somefile

# This command will be blocked
$ cmdjail -- 'rm -rf /'
[error] implicitly blocked intent cmd: rm -rf /
```

#### Example 2: Black and whitelist

**Goal:** Allow a user to run any `git` command except for `git push`.

This configuration uses a deny rule to block a specific command and a broad allow rule to permit others.

`.cmd.jail`:

```diff
# Explicitly deny 'git push'
- 'git push

# Allow any other command that starts with 'git '
+ r'^git
```

**Usage:**

```bash
# This command will be executed
$ cmdjail -- 'git status'
On branch main
Your branch is up to date with 'origin/main'.

# This command will be blocked by the deny rule
$ cmdjail -- 'git push'
[warn] blocked blacklisted intent cmd: git push
```

#### Example 3: Whitelisting Safe Subcommands with grep

**Goal:** Allow a user to view running docker containers and check logs, but prevent them from running, stopping, or building new containers.

This uses a grep command matcher to check that the intent command starts with either docker ps or docker logs. The -qE flags make grep silent (-q) and
enable extended regular expressions (-E). grep exits with code 0 (success) only if a match is found.

`.cmd.jail`:

```diff
# Allow only 'docker ps' and 'docker logs' commands.
# The regex is anchored (^) to ensure the command starts with 'docker'.
+ grep -qE '^docker (ps|logs)'
```

**Usage:**

```bash
# This command will be allowed by the grep matcher
$ cmdjail -- 'docker ps -a'
CONTAINER ID   IMAGE     COMMAND   CREATED   STATUS    PORTS     NAMES
...

# This command will be blocked because it doesn't match the regex
$ cmdjail -- 'docker run -it ubuntu'
[error] implicitly blocked intent cmd: docker run -it ubuntu
```

#### Example 4: Validating Command Arguments with awk

**Goal:** Allow a user to use echo to print messages, but only if the message consists of simple, alphanumeric words. This prevents arguments that contain
shell metacharacters like ;, |, or >.

This awk one-liner checks two conditions: 1) The command must start with echo. 2) Every subsequent argument must only contain letters and numbers. If
both conditions are met, it exits with 0 (success); otherwise, it exits with 1 (failure).

.cmd.jail:

```diff
# Allow 'echo' only with safe, alphanumeric arguments.
+ awk '{ if ($1 != "echo") exit 1; for (i=2; i<=NF; i++) { if ($i !~ /^[a-zA-Z0-9]+$/) exit 1 }; exit 0 }'
```

**Usage:**

```bash
# This command is allowed because all arguments are alphanumeric
$ cmdjail -- 'echo hello world'
hello world

# This command is blocked by the awk script because of the semicolon
$ cmdjail -- 'echo hello; ls'
[error] implicitly blocked intent cmd: echo hello; ls
```

#### Example 5: Allowing cat Safely by Denying Sensitive Paths

**Goal:** Allow a user to read files using cat, but explicitly block them from accessing any file within the /etc/ directory.

cmdjail processes deny rules first. The first rule blocks any cat command where the path contains /etc/. If that rule doesn't match, cmdjail proceeds to
the allow rule, which permits any other cat command.

`.cmd.jail`:

```diff
# 1. Deny any attempt to cat a file in /etc/
- grep -qE '^cat .*/etc/'

# 2. Allow any other 'cat' command.
+ grep -qE '^cat '
```

**Usage:**

```bash
# This command is allowed because it passes the deny rule and matches the allow rule
$ cmdjail -- 'cat /home/user/notes.txt'
Some notes...

# This command is blocked by the first (deny) rule
$ cmdjail -- 'cat /etc/passwd'
[warn] blocked blacklisted intent cmd: cat /etc/passwd
```

### Recording Mode

`cmdjail` includes a recording mode that simplifies the process of building a new jail file. When you run `cmdjail` with the `--record <filepath>` flag, it will:

1.  Execute the intent command immediately, bypassing all rule checks.
2.  Append the executed command to the specified `<filepath>` as a literal allow rule (`+ '...'`).

This is useful for populating a new `.cmd.jail` file by performing the allowed actions once.

**Usage:**

```sh
# This will run 'git status' and add "+ 'git status" to ./my-new.jail
cmdjail --record ./my-new.jail -- 'git status'

# This will run 'ls -l' and add "+ 'ls -l" to the same file
cmdjail --record ./my-new.jail -- 'ls -l'
```

> **Warning:** Recording mode executes commands without validation. Only use it in a trusted environment to build your initial ruleset.

### Interactive Shell Mode

`cmdjail` can run as an interactive, restricted shell. This is useful for providing users with limited, interactive access to a system where every command they type is validated against the jail file.

If no intent command is provided via arguments (`-- '...'`) or environment variables (`CMDJAIL_CMD`), `cmdjail` will automatically start in interactive shell mode.

**Usage:**

```bash
# Start the interactive shell using the rules in ./.cmd.jail
$ cmdjail
```

Once started, `cmdjail` will display a prompt (`cmdjail> `). You can type commands, and they will be executed if allowed by the rules.

```bash
$ cmdjail
cmdjail> ls -l
total 8
-rw-r--r-- 1 user group 12 Jun 1 12:00 .cmd.jail
cmdjail> rm /etc/hosts
[warn] implicitly blocked intent cmd: rm /etc/hosts
cmdjail> exit
```

To exit the shell, type `exit` or `quit`, or press `Ctrl+D`.
