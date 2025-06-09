package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/kelseyhightower/envconfig"
	"github.com/spf13/pflag"
)

const (
	EnvPrefix    = "CMDJAIL"
	JailFilename = ".cmd.jail"
)

var (
	flagLogFile     string
	flagLogFileDesc string = "Path for logging. Set to \"\" for syslog. If unset, logging is disabled."

	flagEnvReference     string
	flagEnvReferenceDesc string = "Environment variable with the intent command (e.g., SSH_ORIGINAL_COMMAND)."

	flagJailFile     string
	flagJailFileDesc string = "Path to the jail file. Use - to read from stdin."

	flagVerbose     bool
	flagVerboseDesc string = "Enable verbose logging for debugging."

	flagRecordFile     string
	flagRecordFileDesc string = "Enable record mode. Executes command and appends it as an allow rule to the file."

	flagVersion     bool
	flagVersionDesc string = "Print version information and exit."

	flagCheck               bool
	flagCheckDesc           string = "Validate jailfile syntax."
	flagCheckIntentCmds     string
	flagCheckIntentCmdsDesc string = "Path to a file with commands to test (use '-' for stdin)."

	flagShellCmd     string
	flagShellCmdDesc string = "Shell command to execute intent commands with."
)

var (
	// TODO: promote to type and capture cmd to log
	ErrCmdNotWrappedInQuotes = errors.New("cmd must be wrapped in single quotes")
	// TODO: promote to type and capture cmd to log
	ErrJailFileManipulationAttempt = fmt.Errorf("attempting to manipulate: %s. Aborted", JailFilename)
	// TODO: promote to type and capture cmd to log
	ErrJailBinaryManipulationAttempt = fmt.Errorf("attempting to manipulate: %s. Aborted", filepath.Base(os.Args[0]))
	// TODO: promote to type and capture cmd to log
	ErrJailLogManipulationAttempt    = errors.New("attempting to manipulate cmdjail log. Aborted")
	ErrJailFileAndCheckCmdsFromStdin = errors.New("jail file and check commands cannot both be read from stdin")
)

type envVars struct {
	IntentCmd    string `envconfig:"CMDJAIL_CMD"`
	LogFile      string
	EnvReference string `envconfig:"CMDJAIL_ENV_REFERENCE"`
	JailFile     string
	RecordFile   string
	Verbose      bool
	ShellCmd     string `envconfig:"CMDJAIL_SHELL_CMD"`
}

func defaultEnvVars() (envVars, error) {
	ex, err := os.Executable()
	if err != nil {
		return envVars{}, err
	}
	exPath := filepath.Dir(ex)

	return envVars{
		JailFile: filepath.Join(exPath, JailFilename),
		ShellCmd: "bash -c",
	}, nil
}

type Config struct {
	IntentCmd           string
	Log                 string
	JailFile            string
	Verbose             bool
	RecordFile          string
	Shell               bool
	Version             bool
	CheckMode           bool
	CheckIntentCmdsFile string
	ShellCmd            []string
}

var NoConfig = Config{}

func init() {
	pflag.ErrHelp = errors.New("")
	pflag.Usage = func() {
		printMsg(os.Stderr, `cmdjail: A flexible, rule-based command filtering proxy.

Acts as an intermediary for executing shell commands. It evaluates a command
against a set of rules in a "jail file" and decides whether to execute or
block it. This is useful for restricting user actions in controlled environments.

Usage:
  cmdjail [flags] -- 'command to execute'
  cmdjail [flags]

When no command is provided, cmdjail starts an interactive shell.

Flags:
  -j, --jail-file <path>         `+flagJailFileDesc+`
                                 (Default: .cmd.jail in binary's directory)
                                 (Env: CMDJAIL_JAILFILE)
  -l, --log-file <path>          `+flagLogFileDesc+`
                                 (Env: CMDJAIL_LOGFILE)
  -e, --env-reference <var>      `+flagEnvReferenceDesc+`
                                 (Env: CMDJAIL_ENV_REFERENCE)
  -r, --record-file <path>       `+flagRecordFileDesc+`
                                 (Env: CMDJAIL_RECORDFILE)
  -s, --shell-cmd <cmd>          `+flagShellCmdDesc+`
                                 (Default: "bash -c")
                                 (Env: CMDJAIL_SHELL_CMD)
  -c, --check                    `+flagCheckDesc+`
      --check-intent-cmds <path> `+flagCheckIntentCmdsDesc+`
  -v, --verbose                  `+flagVerboseDesc+`
                                 (Env: CMDJAIL_VERBOSE)
      --version                  `+flagVersionDesc+`
  -h, --help                     Show this help message.

The intent command can also be set directly via the CMDJAIL_CMD environment variable.`)
	}
}

func parseFlags(envvars envVars) []string {
	pflag.BoolVar(&flagVersion, "version", false, flagVersionDesc)
	pflag.BoolVarP(&flagVerbose, "verbose", "v", envvars.Verbose, flagVerboseDesc)
	pflag.StringVarP(&flagLogFile, "log-file", "l", envvars.LogFile, flagLogFileDesc)
	pflag.StringVarP(&flagEnvReference, "env-reference", "e", envvars.EnvReference, flagEnvReferenceDesc)
	pflag.StringVarP(&flagJailFile, "jail-file", "j", envvars.JailFile, flagJailFileDesc)
	pflag.StringVarP(&flagRecordFile, "record-file", "r", envvars.RecordFile, flagRecordFileDesc)
	pflag.BoolVarP(&flagCheck, "check", "c", false, flagCheckDesc)
	pflag.StringVar(&flagCheckIntentCmds, "check-intent-cmds", "", flagCheckIntentCmdsDesc)
	pflag.StringVarP(&flagShellCmd, "shell-cmd", "s", envvars.ShellCmd, flagShellCmdDesc)

	args, cmdOptions := splitAtEndOfArgs(os.Args)
	pflag.CommandLine.Parse(args)

	return cmdOptions
}

func parseEnvVars() (envVars, error) {
	envvars, err := defaultEnvVars()
	if err != nil {
		return envVars{}, err
	}

	err = envconfig.Process(EnvPrefix, &envvars)
	if err != nil {
		return envVars{}, err
	}

	return envvars, nil
}

// It returns the effective log file path (or "" for syslog/discard) and any error.
func initializeLogging(envLogFile, cliLogFile string) (string, error) {
	logFileIsSetByFlag := pflag.CommandLine.Changed("log-file")
	_, logFileIsSetByEnv := os.LookupEnv(EnvPrefix + "_LOGFILE")

	var effectiveLogPath string
	if !logFileIsSetByEnv && !logFileIsSetByFlag {
		log.SetOutput(io.Discard)
		printLogDebug(os.Stdout, "logging disabled")
	} else {
		effectiveLogPath = envLogFile
		if logFileIsSetByFlag {
			effectiveLogPath = cliLogFile
		}

		if effectiveLogPath == "" {
			// An empty value means use syslog.
			if err := setLoggerToSyslog(); err != nil {
				return "", fmt.Errorf("configuring syslog logger: %w", err)
			}
			printLogDebug(os.Stdout, "logging to syslog")
		} else {
			// A non-empty value is a file path.
			if err := setLoggerToFile(effectiveLogPath); err != nil {
				return "", fmt.Errorf("configuring file logger: %w", err)
			}
			printLogDebug(os.Stdout, "logging to: %s", effectiveLogPath)
		}
	}
	return effectiveLogPath, nil
}

func resolveIntentCommand(envCmd, envRef string, cliCmdOptions []string) (string, error) {
	cmd := envCmd
	if cmd != "" {
		printLogDebug(os.Stderr, "intent command loaded from: $%s_CMD", EnvPrefix)
	}

	if cmd == "" && envRef != "" {
		cmd = os.Getenv(envRef)
		printLogDebug(os.Stderr, "intent command loaded from: $%s", envRef)
	}

	if len(cliCmdOptions) > 0 {
		if len(cliCmdOptions) > 1 {
			return "", ErrCmdNotWrappedInQuotes
		}
		cmd = cliCmdOptions[0]
		printLogDebug(os.Stderr, "intent command loaded from arguments")
	}
	return cmd, nil
}

func determineOperationMode(intentCmd string, checkFlag bool, checkIntentCmdsFile string) (shellMode bool, checkMode bool) {
	checkMode = checkFlag || checkIntentCmdsFile != ""
	shellMode = intentCmd == "" && !checkMode
	return shellMode, checkMode
}

func parseEnvAndFlags() (Config, error) {
	envvars, err := parseEnvVars()
	if err != nil {
		return NoConfig, err
	}

	cmdOptions := parseFlags(envvars)

	if flagJailFile == "-" && flagCheckIntentCmds == "-" {
		return NoConfig, ErrJailFileAndCheckCmdsFromStdin
	}

	if flagVerbose {
		debug = true
	}

	logPathOrSyslog, err := initializeLogging(envvars.LogFile, flagLogFile)
	if err != nil {
		return NoConfig, err
	}

	printLogDebug(os.Stderr, "loaded env vars: %+v", envvars)
	pflag.Visit(func(f *pflag.Flag) {
		printLogDebug(os.Stderr, "flag set: %+v", f)
	})

	if envvars.IntentCmd != "" && envvars.EnvReference != "" {
		printLogWarn(os.Stderr, "both %s and %s environment variables are set", EnvPrefix+"_CMD")
	}

	intentCmd, err := resolveIntentCommand(envvars.IntentCmd, flagEnvReference, cmdOptions)
	if err != nil {
		return NoConfig, err
	}

	shellMode, checkMode := determineOperationMode(intentCmd, flagCheck, flagCheckIntentCmds)

	if intentCmd != "" {
		if err := checkCmdSafety(intentCmd, logPathOrSyslog); err != nil {
			return NoConfig, err
		}
	}

	shellCmdFields := strings.Fields(flagShellCmd)
	if len(shellCmdFields) == 0 {
		shellCmdFields = []string{"bash", "-c"} // Fallback
	}

	return Config{
		IntentCmd:           intentCmd,
		Log:                 logPathOrSyslog,
		JailFile:            flagJailFile,
		Verbose:             flagVerbose,
		RecordFile:          flagRecordFile,
		Shell:               shellMode,
		Version:             flagVersion,
		CheckMode:           checkMode,
		CheckIntentCmdsFile: flagCheckIntentCmds,
		ShellCmd:            shellCmdFields,
	}, nil
}

func checkCmdSafety(cmd, logPath string) error {
	if strings.Contains(cmd, JailFilename) {
		return ErrJailFileManipulationAttempt
	} else if strings.Contains(cmd, filepath.Base(os.Args[0])) {
		return ErrJailBinaryManipulationAttempt
	} else if logPath != "" && strings.Contains(cmd, logPath) {
		return ErrJailLogManipulationAttempt
	}
	return nil
}

func splitAtEndOfArgs(args []string) ([]string, []string) {
	if len(args) == 0 || len(args) == 1 {
		return nil, nil
	}

	args = args[1:]

	for i, arg := range args {
		if arg == "--" {
			return args[:i], args[i+1:]
		}
	}
	return args, nil
}
